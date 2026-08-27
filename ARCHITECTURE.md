# 架构设计文档

> browser-env-sandbox · Go + V8 架构
> 2026-08-27 重构（从 Node.js vm 升级）

## 1. 技术选型

### 1.1 核心语言：Go

**为什么选 Go：**
- v8go (rogchap/v8go) — 成熟的 Go V8 binding，纯 V8 Isolate
- goroutine 并发 — 天然适合 Isolate 池化 + 多会话并发
- gRPC 生态 — 原计划 gRPC，已改为 JSON-over-HTTP（Go 1.22 ServeMux），零 protoc 依赖
- 编译为单二进制 — 部署简单，无运行时依赖
- CGO — v8go 通过 CGO 调 V8，同时可 FFI 桥接 curl-impersonate

**为什么不用 Node.js vm（v1 方案）：**
- vm 不是真沙箱，有已知逃逸路径
- vm 外层是 Node 进程，宿主污染无法根除
- 单线程事件循环，无法池化并发
- 桥接只能子进程，开销大

**为什么不用 Rust + rusty_v8：**
- 欧尼酱本地有 Go 环境，无 Rust 环境
- Go 的开发效率在原型阶段更高
- v8go 足够成熟，Isolate 隔离能力与 rusty_v8 等价
- 未来如需极致性能可迁移 Rust（接口不变）

### 1.2 V8 接入：v8go

```go
import "github.com/rogchap/v8go"

// 创建 Isolate（独立堆，GC 不共享）
iso := v8go.NewIsolate()
defer iso.Dispose()

// 创建 Context
ctx := v8go.NewContext(iso)
defer ctx.Close()

// 注入全局对象
ctx.Global().Set("navigator", navigatorObj)
ctx.Global().Set("document", documentObj)

// 执行 JS
val, err := ctx.RunScript(code, "target.js")
```

**Isolate 池化设计：**
```
Isolate Pool (sync.Pool or channel-based)
├── iso[0] — 预热, 指纹已灌入
├── iso[1] — 预热, 指纹已灌入
├── iso[2] — 空闲
└── iso[3] — 执行中 (session #42)
```

## 2. 五大核心模块

### 2.1 指纹引擎 (fpengine)

**职责：** 程序化生成自洽的浏览器指纹，不是 dump，是合成。

**核心挑战 — 自洽性：**

指纹不能随机拼凑。如果 UA 说 Chrome 131 + Windows 11，那以下必须全部一致：

| 属性 | 约束 |
|------|------|
| navigator.userAgent | Chrome 131, Windows |
| navigator.platform | Win32 |
| navigator.userAgentData.brands | [{Chromium, 131}, {Google Chrome, 131}, {Not.A/Brand, 24}] |
| screen.colorDepth | 24 (Windows 标准) |
| WebGL UNMASKED_RENDERER | ANGLE (NVIDIA, RTX 4060 ...) 或 Intel UHD |
| canvas toDataURL hash | 与 GPU + 字体集 + Chrome 版本自洽 |
| 字体集 | Windows 默认字体 + GPU 驱动字体 |
| AudioContext fingerprint | 与 OS + 浏览器版本自洽 |
| Intl.DateTimeFormat | 与 IP 地理位置时区一致 |
| navigator.languages | 与 IP 地理 + UA 语言一致 |

**架构：**

```
fpengine/
├── engine.go          # 引擎入口: Generate(opts) → Fingerprint
├── knowledge_base.go  # 知识库: 浏览器×OS×硬件 矩阵
├── navigator.go       # navigator 属性合成
├── screen.go          # screen 属性合成
├── canvas.go          # canvas 指纹合成
├── webgl.go           # WebGL 指纹合成
├── audio.go           # AudioContext 指纹合成
├── fonts.go           # 字体集合成
├── timezone.go        # 时区合成 (与 IP 地理联动)
├── webrtc.go          # WebRTC IP 指纹
└── consistency.go     # 自洽性校验
```

**知识库结构：**

```go
type KnowledgeBase struct {
    Browsers  []BrowserProfile  // Chrome 120-135, Firefox, Safari
    OS        []OSProfile       // Win10/11, macOS, Linux, Android, iOS
    GPUs      []GPUProfile      // NVIDIA RTX 3060-4090, Intel UHD/Iris, AMD Radeon
    Fonts     map[OS][]string   // 各 OS 默认字体集
    Screens   []ScreenProfile   // 常见分辨率组合
    Timezones []TimezoneProfile // 时区+地区
}
```

**生成流程：**

```
1. 种子 → 确定性 RNG (相同种子 = 相同指纹)
2. 从知识库采样: 浏览器 + OS + GPU + 屏幕 + 时区
3. 自洽约束传播:
   OS=Windows → platform=Win32, 字体=Win字体集
   GPU=RTX4060 → WebGL renderer=NVIDIA, canvas hash 匹配
   时区=Asia/Shanghai → Intl, languages=["zh-CN","zh"]
4. 一致性校验 → 通过则输出, 否则重新采样
```

**指纹输出格式：**

```go
type Fingerprint struct {
    Seed       uint64
    Browser    BrowserProfile
    OS         OSProfile
    GPU        GPUProfile
    Navigator  map[string]interface{}
    Screen     map[string]interface{}
    Canvas     CanvasFingerprint   // toDataURL hash + measureText
    WebGL      WebGLFingerprint    // vendor, renderer, extensions, params
    Audio      AudioFingerprint    // AudioContext hash
    Fonts      []string            // 可探测字体列表
    Timezone   string              // Asia/Shanghai
    Languages  []string            // ["zh-CN", "zh"]
    Window     WindowProps         // innerWidth, etc.
}
```

### 2.2 V8 沙箱引擎 (sandbox)

**职责：** 接收指纹 + 目标 JS，在纯 V8 Isolate 中构建浏览器环境并执行。

**浏览器 API mock 策略：**

在 Go 侧构建 mock 对象，通过 v8go 注入 V8 context。不是用 JS 写 mock，而是 Go 原生注入——更难被检测。

```go
// Go 侧构建 navigator 对象
nav := v8go.NewObject(iso)
nav.Set("userAgent", fp.Navigator["userAgent"])
nav.Set("platform", fp.Navigator["platform"])
nav.Set("webdriver", false)
// ... 注入全套属性

// 注入到 context 全局
global := ctx.Global()
global.Set("navigator", nav)
global.Set("window", global)    // window = globalThis
global.Set("self", global)
global.Set("top", global)       // 不用 Proxy，直接引用
```

**模块结构：**

```
sandbox/
├── engine.go          # 沙箱引擎: Create(session) → Sandbox
├── isolate_pool.go    # Isolate 池化: 预热/复用/回收
├── browser_env.go     # 浏览器环境构建 (Go → V8 注入)
├── dom_mock.go        # DOM 最小子集 (Go 实现)
├── timers.go          # 事件循环模拟
├── console.go         # console.log/error/warn mock
└── tracing.go         # 执行追踪 (first divergence 定位)
```

**Isolate 池化：**

```go
type IsolatePool struct {
    isolates chan *v8go.Isolate  // 带缓冲 channel
    factory  func() *v8go.Isolate
}

func (p *IsolatePool) Get() *v8go.Isolate {
    select {
    case iso := <-p.isolates:
        return iso  // 复用
    default:
        return p.factory()  // 新建
    }
}

func (p *IsolatePool) Put(iso *v8go.Isolate) {
    // 重置 context，保留 isolate
    p.isolates <- iso
}
```

**Node 痕迹抹除（v2 优势）：**

v8go 创建的 Isolate **天然没有** Buffer/process/require/module。这是相比 Node vm 的根本优势——不是「抹除」，而是「从不存在」。

### 2.3 网络层 (netlayer)

**职责：** 沙箱内网络请求的处理——离线 replay + 在线转发 + session-unique。

**双模式：**

```
模式 1: 离线 Replay
┌──────┐     ┌──────────┐     ┌──────────┐
│沙箱JS │ XHR │ Replay   │ 查找 │ Recording│
│      │────▶│ Handler  │────▶│  Store   │
│      │     │          │     │ (JSON)   │
│      │◀────│          │◀────│          │
└──────┘     └──────────┘     └──────────┘
  不发真请求，从录制中取响应

模式 2: 在线转发
┌──────┐     ┌──────────┐     ┌──────────────┐
│沙箱JS │ XHR │ Forward  │ HTTP│ curl-impersonate│
│      │────▶│ Handler  │────▶│ (TLS 指纹匹配) │
│      │     │          │     │ + 代理 IP     │
│      │◀────│          │◀────│              │
└──────┘     └──────────┘     └──────────────┘
  发真请求，TLS 指纹与 UA 一致
```

**模块结构：**

```
netlayer/
├── handler.go        # XHR/fetch 拦截 + 分发
├── replay.go         # 离线 replay 引擎
├── forward.go        # 在线转发 (curl-impersonate)
├── recording.go      # 请求录制 + 存储
├── cookie_jar.go     # Cookie 管理 (与 document.cookie 联动)
├── tls_profile.go    # TLS 指纹配置 (JA3/JA4)
└── proxy.go          # 代理管理 (per-session)
```

**Session-Unique 实现：**

```go
type NetworkSession struct {
    ID          string
    Fingerprint *Fingerprint      // 绑定指纹
    TLSProfile  *TLSProfile       // JA3 与 UA 版本一致
    CookieJar   *CookieJar        // 独立 cookie
    Proxy       string            // 独立代理 IP
    Recording   *Recording        // 可选: 录制本 session 请求
}
```

多账号操作时，每个 session 的 TLS 指纹、cookie、IP、UA 全部不同，零关联。

### 2.4 调试层 (debug)

**职责：** 暴露 CDP (Chrome DevTools Protocol) 接口，让真 Chrome DevTools 能连上来调试沙箱内的 JS。

**核心能力：**
- `chrome://inspect` → 连接沙箱
- Network panel → 看沙箱内的 XHR/fetch 请求
- Sources panel → 断点、步进、变量查看
- Console → 沙箱内的 console.log 输出
- Performance → 执行时间分析

**模块结构：**

```
debug/
├── cdp_server.go      # CDP WebSocket 服务器
├── cdp_domains.go     # CDP 域实现 (Network/Runtime/Debugger)
├── request_capture.go # 请求采集 (与 netlayer 联动)
├── breakpoint.go      # 断点管理
└── console.go         # console 消息转发
```

**CDP 域映射：**

| CDP 域 | 功能 | 实现 |
|--------|------|------|
| Runtime | evaluate, callFunctionOn | v8go ctx.RunScript |
| Debugger | pause, resume, setBreakpoint | v8go debugger API |
| Network | requestWillBeSent, responseReceived | netlayer 事件转发 |
| Console | messageAdded | sandbox console mock |
| Page | navigate, getNavigationHistory | location mock |

### 2.5 桥接层 (bridge)

**职责：** JSON-over-HTTP API + SSE 流 + 多语言 SDK，让任何语言都能用沙箱。

**架构：**

```
                    JSON-over-HTTP + SSE
Go Core ◄──────────────────────────────────────► Python SDK
  │                                                Go SDK
  │                                                Node SDK
  │                                                CLI (bes)
```

**API 端点：**

```
GET    /api/session                       list sessions
POST   /api/session                       create session (supports preload + init)
POST   /api/session/{id}/eval             evaluate JS
POST   /api/session/{id}/script           load & run a named script
POST   /api/session/{id}/call             call a global function
GET    /api/session/{id}/fingerprint      get full fingerprint
GET    /api/session/{id}/cookies          get cookie jar
POST   /api/session/{id}/cookies          set a cookie
DELETE /api/session/{id}                  close session
GET    /api/session/{id}/stream/console   SSE stream of console messages
GET    /api/session/{id}/stream/network   SSE stream of network events
GET    /health                            liveness probe
```

**模块结构：**

```
bridge/
├── server.go         # HTTP API 服务器 (Go 1.22 ServeMux)
├── service.go        # 业务逻辑层 (session 注册表 + broadcaster)
└── proto/
    └── sandbox.proto  # 已废弃 (仅参考)
```

**SDK 结构：**

```
sdk/
├── python/
│   ├── bes/__init__.py    # from bes import Sandbox
│   ├── bes/client.py      # HTTP 客户端
│   └── bes/fingerprint.py # 指纹辅助
├── go/
│   └── bes.go             # import "bes/sdk/go"
├── node/
│   └── bes.js             # const { Sandbox } = require('bes')
└── cli/
    └── bes/               # bes CLI (Go 编译)
```

## 3. Session 生命周期

```
创建 Session
  │
  ├── 1. 指纹引擎生成指纹 (种子 or 随机)
  ├── 2. 网络层创建 session (TLS + cookie + proxy)
  ├── 3. 沙箱引擎从池中取 Isolate
  ├── 4. 指纹灌入 + 浏览器 API 注入
  │
  ▼
Session 就绪
  │
  ├── eval(code)          → 执行 JS
  ├── loadScript(file)    → 加载脚本
  ├── callFunction(...)   → 调用函数
  ├── getFingerprint()    → 查看指纹
  ├── streamConsole()     → console 流
  ├── streamNetwork()     → 网络流
  │
  ▼
关闭 Session
  │
  ├── Isolate 重置 → 归还池
  ├── Cookie jar 持久化 (可选)
  └── 网络资源释放
```

## 4. 项目结构

```
browser-env-sandbox/
├── go.mod
├── README.md
├── ARCHITECTURE.md           ← 本文件
├── docs/
│   ├── design-constraints.md ← 浏览器环境模拟实战经验
│   ├── roadmap.md
│   └── fingerprint-kb.md     ← 指纹知识库文档
├── cmd/
│   ├── bes/                  ← CLI 入口
│   │   └── main.go
│   └── bes-server/           ← HTTP API 服务入口
│       └── main.go
├── internal/
│   ├── fpengine/             ← 指纹引擎
│   ├── sandbox/              ← V8 沙箱引擎
│   ├── netlayer/             ← 网络层
│   ├── debug/                ← 调试层 (CDP)
│   └── bridge/               ← JSON-over-HTTP 桥接
├── pkg/
│   └── api/                  ← 公共 API 类型
├── sdk/
│   ├── python/
│   ├── go/
│   └── node/
```

## 5. 与逆向工作流的集成

```
Observe → Capture → Rebuild → Patch → DeepDive
                     │          │        │
                     │   ┌──────┘        │
                     │   │               │
                     ▼   ▼               ▼
              ┌─────────────┐    ┌──────────────┐
              │ 沙箱 Rebuild  │    │ 调试层 CDP    │
              │ 指纹灌入      │    │ 断点+请求采集  │
              │ 目标 JS 执行   │    │ 去混淆分析     │
              └─────────────┘    └──────────────┘
```

## 6. 设计约束

以下 10 条实战经验适用于本项目，且 Go + v8go 架构天然解决其中几条：

| 约束 | Node.js vm | Go + v8go |
|------|-------------|----------------|
| Node 痕迹抹除 | 需手动设 undefined | **天然不存在** |
| top/parent 不用 Proxy | 手动自引用 | Go 注入直接引用 |
| toString 伪装 | JS Object.defineProperty | Go 侧设置 toStringTag |
| navigator 冻结 | Object.freeze | Go 侧只读注入 |
| canary 探针 | 不用 Proxy | Go 注入, 不存在的属性天然 undefined |

详见 [docs/design-constraints.md](docs/design-constraints.md)
