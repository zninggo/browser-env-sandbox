# 双执行器架构：V8 沙箱 + 真 Chrome CDP

> browser-env-sandbox · `internal/executor`
> 对应父任务：ZMTO-45 优化项 3

## 1. 为什么需要双执行器

纯 V8 沙箱对「数据型指纹点」（UA、platform、屏幕、GPU 字符串、字体列表）可以做到
自洽伪造，但对**需要真实执行的指纹点**有天花板：

| 指纹点 | 纯 V8 的问题 |
|---|---|
| canvas | toDataURL 是回放的预采集值；换场景（画布尺寸/文字内容变化）即露馅 |
| WebGL readback | readPixels 回放固定 buffer；非预采集场景返回异常 |
| AudioContext | DSP 输出是 mock 值，无法通过真实性统计检验 |
| WebAssembly | 引擎特征面（threads/SIMD/异常提案）无法伪造 |

双执行器架构 = **默认 V8（快、无痕）+ 按指纹点类型桥接真 Chrome（真实渲染）**，
核心不变式：**同一份 device profile 驱动所有执行器** —— 身份面（UA、platform、
GPU 字符串、屏幕、时区、语言、userAgentData）在两个执行器下读数完全一致，差异只
出现在执行引擎本身（canvas/wasm 结果为真）。

## 2. 组成

```
internal/executor/
├── executor.go    Executor / Session 接口 + PointClass 指纹点分类
├── profile.go     ProfileSnapshot —— 共享的 device profile 快照
├── v8.go          V8Executor：包装 sandbox.Engine（默认执行器）
├── chrome.go      ChromeExecutor：CDP 注入执行器（页面级 target）
├── chrome_launcher.go  本地隔离 Chrome 的发现/启动/回收
├── cdp.go         极简 CDP 客户端（RFC 6455 ws + JSON-RPC 命令关联）
├── inject.go      profile → Chrome 注入 JS（与 V8 env 同源同值）
├── probe.go       身份探针 + canvas/wasm 真实性探针
├── router.go      路由/回退（切换机制）
└── consistency.go profile 一致性校验（VerifyConsistency）
```

### 2.1 ProfileSnapshot —— 统一 device profile 驱动接口

```go
snap, _ := executor.TakeSnapshot(fpEngine, seed, browser, os, timezone)
// snap.Fingerprint 是 fpengine 的一次联合实现（UA↔GPU↔屏幕↔字体↔核数↔内存↔时区联合采样）
```

- V8 执行器：`sandbox.Engine.CreateSessionWithFingerprint(opts, snap.Fingerprint)`
  把**同一个 Fingerprint 对象**灌进 V8 环境（新增入口，`CreateSession` 语义不变）。
- Chrome 执行器：同一 Fingerprint 翻译成 CDP 调用 + 注入脚本（见 2.3）。

一致性由构造保证：两边读的都是 `snap.Fingerprint` 的同一批字段，不存在二次采样。

### 2.2 PointClass —— 指纹点分类

| class | 说明 | 路由 |
|---|---|---|
| `js` | 纯逻辑（签名算法等） | v8 |
| `navigator` | navigator 身份串 | v8 |
| `screen` | 屏幕/窗口几何 | v8 |
| `fonts` | 字体枚举 | v8（本地 Chrome 会泄漏宿主 Linux 字体，刻意留在 V8） |
| `canvas` | 真实栅格化 | **chrome**（可用时） |
| `webgl` | 真实 GL 渲染（身份串仍取 profile） | **chrome** |
| `audio` | 真实 AudioContext DSP | **chrome** |
| `wasm` | 真实 wasm 引擎 | **chrome** |

`Router.Route(class)` 是纯决策（无副作用、可并发）；`Router.Eval(...)` 执行并
自动回退（见 §3）。

### 2.3 Chrome CDP 执行器的 profile 注入

**CDP 原生层**（浏览器必须端到端一致的）：

- `Emulation.setUserAgentOverride` — UA + platform + acceptLanguage（连带 HTTP 头、
  iframe 内一致）
- `Emulation.setTimezoneOverride` — Date/Intl 全部生效（比 V8 的 JS 包装更强）

**注入脚本层**（`Page.addScriptToEvaluateOnNewDocument` + 当前文档立即执行，
即 `BuildInjectionJS(fp)`）：

- navigator 派生/附加身份：appVersion、vendor、languages、hardwareConcurrency、
  deviceMemory、maxTouchPoints、userAgentData（非 Chromium profile 置
  undefined，与 V8 的浏览器调整一致）、connection
- screen / window 几何：width/height/avail*/colorDepth/orientation、
  inner/outer 尺寸、devicePixelRatio
- WebGL **身份常量**：`getParameter` 打补丁 —— 37445/37446（UNMASKED vendor/
  renderer）、VERSION、SHADING_LANGUAGE_VERSION 等 profile 值；**其余 pname 直通
  真实 GL 驱动**（这是 Chrome 执行器存在的意义：渲染是真的，身份是 profile 的）

**刻意不注入**：canvas、AudioContext、WebAssembly —— 它们必须真实执行；字体
（会泄漏宿主字体，路由到 V8）。

## 3. 切换与回退策略

```
                 ┌─────────────────────────────────────────┐
   Eval(class) ──► Route(class)                            │
                 └── real class && chrome.Available()? ────┤
                        yes │                    no        │
                            ▼                        ▼                     │
                     Chrome 执行器                 V8 执行器（默认路径，永不变）
                            │
                    失败/超时? ──yes──► 回退 V8 重执行（记录 fallback 原因，
                            │           OnFallback 回调 + Stats 计数）
                          no▼
                       返回 chrome 结果
```

- **默认快路径零开销**：Chrome 不存在/不可用时，一切类都走 V8，没有任何启动、
  探测成本（`Available()` 只是二进制存在性检查）。
- **回退语义**：real 类在 Chrome 上失败（启动失败、session 失败、eval 错误、
  超时）→ 自动回退 V8 用 mock 值完成本次求值，调用方拿到结果的同时通过
  `EvalResult.Executor`（`"v8(fallback:chrome)"`）与 `EvalResult.Fallback`
  知道发生了回退。
- **SessionFor 不做静默回退**：长生命周期 session 的创建失败显式报错 ——
  canvas 点悄悄落到 mock 路径上比报错更危险。
- **长生命周期复用**：`Router.Eval` 每次调用建/销一个 session（Chrome 侧是
  共享进程上的一个 page target，~几十 ms）；批量场景用 `SessionFor` 拿长连
  接 session 自己管理。

## 4. 一致性校验（profile 一致性校验逻辑）

`VerifyConsistency(ctx, snap, v8, chrome)`：

1. V8 session + Chrome session 各自跑**身份探针**（同一份
   `identityProbeJS`，读 28 个身份字段）。
2. 与 `ExpectedIdentity(fp)`（profile 期望值）逐字段比对 —— 通过即等价于
   三方一致（profile = V8 = Chrome）。
3. canvas/wasm 探针收集真实执行证据：Chrome 的 canvas 必须是「真实 PNG 且体量
   合理」（真实栅格化的 dataURL 远长于 V8 回放值），wasm 必须编译+执行成功。
4. 产出 `ConsistencyReport{Fields, AllPass, Canvas/Wasm 证据, RealPass}`。

CLI 入口：

```bash
bes dual-check [--browser chrome] [--os windows] [--seed 0] [--timezone <tz>] [--json]
```

当前状态（本机 Chrome 152 / arm64 实测）：chrome×windows/macos/linux/android、
firefox×windows、safari×macos 全部 **28/28 身份字段一致，canvas/wasm 真实**。
校验还顺手修掉了两个 V8 环境的真实缺口（见 §6）。

## 5. 安全与隔离约束

- **只连本地**：CDP 拨号层（`validateLocalWSURL` / `validateLocalHTTPURL`）
  白名单 127.0.0.1 / localhost / [::1]，远程调试端点在 dial 层直接拒绝 ——
  无法通过配置指向任何远程/线上浏览器。
- **只启动自己的 Chrome**：执行器不连接已运行的浏览器，只启动自己 fork 的实例
 （临时 user-data-dir、`--remote-debugging-port=0` 随机端口）；关闭时按**精确
  PID** kill（绝不按进程名匹配），并清理临时目录。
- **导航白名单**：session 文档只允许 about:/data: 惰性 URL —— Chrome 执行器
  是指纹点执行器，不做页面抓取（真实流量属于 sandbox+netlayer 职责）。
- 环境变量 `BES_CHROME_BIN` 可覆盖二进制路径（仍要求本地文件）。

## 6. 已知限制

- **字体**：Chrome 执行器不伪装宿主字体；字体枚举点路由到 V8。代价是 Chrome
  里 canvas 文字栅格化用的是宿主字体（Linux 字形）—— 与「NVIDIA + Windows」
  这类 profile 的跨设备一致性存在理论偏差，但 canvas hash 本身不可逆推出
  GPU/OS，风险可控。
- **canvas hash 跨执行器不同**：这是设计而非缺陷 —— V8 回放预采集值、Chrome
  真实栅格化。需要「真值」的场景按 canvas 类路由即可。
- **defineProperty 注入 vs 原型链检测**：注入用 own-property 覆盖（与 V8 env
  同等水平）。读取 `Object.getOwnPropertyDescriptor(Navigator.prototype, ...)`
  的深度检测能看出覆盖痕迹 —— 后续可升级为 Proxy/原型链级注入。
- **每 eval 一个 page target**：高频批量场景应使用长 session（`SessionFor`）。
- headless Chrome 在无 GPU 主机上走 SwiftShader（启动参数
  `--enable-unsafe-swiftshader` 保证 WebGL 可用）—— 渲染真实但非硬件加速。

## 7. 用法

```go
eng := fpengine.New()
v8 := executor.NewV8Executor(eng, 4)
defer v8.Dispose()
chrome := executor.NewChromeExecutor()   // 找不到 Chrome 时 Available()==false
defer chrome.Dispose()

snap, _ := executor.TakeSnapshot(eng, seed, "chrome", "windows", "")

r, _ := executor.NewRouter(v8, chrome)

// 默认路径：V8
res, _ := r.Eval(ctx, executor.PointJS, snap, "sign('x')", opts, 30*time.Second)

// canvas 点：真 Chrome 栅格化；Chrome 挂了自动回退 V8
res, _ = r.Eval(ctx, executor.PointCanvas, snap, canvasFPJS, opts, 30*time.Second)

// 全量一致性校验
rep, _ := executor.VerifyConsistency(ctx, snap, v8, chrome, 30)
```
