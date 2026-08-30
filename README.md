# browser-env-sandbox

> 基于 V8 引擎的浏览器环境模拟平台 —— 程序化生成自洽指纹、纯 V8 高性能执行、可编程浏览器环境

## 这是什么

一个用 **Go + V8** 构建的浏览器环境模拟平台。不是零散的补环境脚本，而是一套完整的浏览器环境模拟生态：

- **指纹引擎** — 程序化生成自洽的浏览器指纹（navigator/canvas/WebGL/Audio/字体/时区），随机但内部一致
- **V8 沙箱** — 纯 V8 Isolate 执行浏览器 JS，零宿主痕迹，Isolate 池化高性能复用
- **网络层** — 离线 replay + 在线转发双模式，session-unique 环境隔离
- **调试层** — 暴露 CDP 协议，真 Chrome DevTools 可直连调试
- **AI Agent 集成** — 内置 MCP server（`bes mcp`），任何支持 MCP 的 AI 客户端可直接驱动沙箱
- **Web 管理台** — Dashboard UI 查看会话、指纹与运行状态
- **多语言桥接** — JSON-over-HTTP 服务 + Python/Go/Node SDK + CLI

> **定位与用途**：浏览器环境模拟服务于自动化测试、前端环境兼容性验证与安全研究（如风控规则的对抗性测试）。使用者需自行遵守目标网站服务条款与所在地法律法规。

## 为什么用 Go + V8

| | Node.js vm | **Go + v8go** |
|---|---|---|
| V8 引擎 | ✅ 但是 Node 包着的 | ✅ 纯 V8 Isolate |
| 宿主污染 | vm 外层是 Node 进程 | **零宿主**，完全可控 |
| 沙箱隔离 | vm 有已知逃逸路径 | V8 Isolate 编译级隔离 |
| 性能 | 单线程事件循环 | **Isolate 池化 + goroutine 并发** |
| 多语言桥接 | 只能子进程 | **JSON-over-HTTP + SDK，任意语言** |
| 内存安全 | JS GC | **Go GC + V8 GC 双层** |

## 架构

```
┌──────────────────────────────────────────────────┐
│              browser-env-sandbox (Go)              │
│                                                    │
│  ┌──────────────┐    指纹引擎                      │
│  │  FP Engine   │    程序化生成自洽指纹             │
│  │  种子→指纹    │    navigator/canvas/WebGL/Audio   │
│  └──────┬───────┘    /字体/时区/WebRTC              │
│         │ 灌入                                     │
│  ┌──────▼───────┐    V8 沙箱引擎                    │
│  │  Sandbox     │    v8go Isolate 池化             │
│  │  Engine      │    浏览器 API mock (Go 注入)      │
│  │              │    零 Node 痕迹                   │
│  └──┬────┬──┬───┘                                 │
│     │    │  │                                     │
│  ┌──▼─┐│┌─▼──┐ ┌──────┐                           │
│  │网络 │││调试 │ │ 桥接  │                           │
│  │Net │││CDP │ │HTTP  │                           │
│  │replay│││   │ │ API  │                           │
│  │+live│││   │ │      │                           │
│  └────┘│└───┘ └──┬───┘                           │
│        │         │                               │
│  ┌─────▼────┐   │  SDK                            │
│  │Session    │  Python / Go / Node / CLI         │
│  │Manager    │                                   │
│  │unique     │                                   │
│  └──────────┘                                   │
└──────────────────────────────────────────────────┘
```

## 快速开始

> **平台要求**：v8go 的预编译 V8 静态库仅支持 **Linux (x86_64/arm64) 和 macOS**。Windows 原生编译不可用，请通过 Docker 或 WSL2 运行（见下方 Docker 部署）。

```bash
# 编译（CGO 必须开启，v8go 通过 CGO 调用 V8）
CGO_ENABLED=1 go build -o bes ./cmd/bes
CGO_ENABLED=1 go build -o bes-server ./cmd/bes-server

# 运行自测套件（224 项浏览器环境检测）
CGO_ENABLED=1 go build -o bes-selftest ./cmd/bes-selftest
./bes-selftest

# 生成指纹（seed 为 uint64，0 = 随机；相同 seed 输出相同指纹）
./bes fingerprint --browser chrome --os windows --seed 42

# 在沙箱中执行 JS
./bes run --eval "navigator.userAgent" --browser chrome --os windows
./bes run --script target.js --location "https://example.com/login"

# 导出指纹到 JSON 文件
./bes export-fp --output fp.json --seed 42

# 启动 HTTP API 服务器（供 SDK 调用）
./bes-server --port 8080 --pool 8

# Docker 部署（Linux 容器，任何宿主平台可用）
docker-compose up -d    # API: 19821, CDP: 9223
```

CLI 完整命令：`fingerprint` / `run` / `export-fp` / `selftest` / `mcp` / `version`。

## Python SDK

Python SDK 通过 JSON-over-HTTP 连接 bes-server，先用 `./bes-server` 或 Docker 启动服务：

```python
from bes import Sandbox

# 连接 bes-server 并创建沙箱 session（browser/os/seed/location 均可指定）
sandbox = Sandbox(server_addr="localhost:8080", browser="chrome", os="windows")
sandbox.eval("navigator.userAgent")                 # → "Mozilla/5.0 ... Chrome/1xx ..."
sandbox.load_script("target.js")                    # 加载并执行脚本
result = sandbox.call("myGlobalFn", "arg1", "arg2") # 调用沙箱内的全局函数
sandbox.fingerprint                                 # 完整指纹 dict
sandbox.set_cookie("k", "v")                        # document.cookie 读写
sandbox.close()

# Context manager 自动释放
with Sandbox() as s:
    print(s.eval("navigator.platform"))
```

## 核心特性

### 指纹自洽

```
种子: 0x3F2A1B...
  → Chrome 131 + Windows 11 + RTX 4060
  → navigator.userAgent = "Mozilla/5.0 ... Chrome/131 ..."
  → navigator.platform = "Win32"
  → WebGL renderer = "ANGLE (NVIDIA, RTX 4060 ...)"
  → canvas hash = 8a3f... (与上述组合自洽)
  → 字体集 = Windows 默认 + NVIDIA 驱动字体
  → 时区 = Asia/Shanghai (IP 地理一致)
```

不是随机拼凑，是**知识库驱动的自洽组合**。

### Session-Unique

每个会话独立：TLS 指纹 + cookie jar + 代理 IP + 指纹 + UA。多测试场景并行时各 session 之间完全隔离。

### 离线 Replay

录制真实浏览器请求序列 → 离线重放给沙箱。沙箱内的 XHR/fetch 不发真请求，而是从录制中取响应。用于反复调试 JS 逻辑而不产生真实流量。

### MCP（AI Agent 集成）

沙箱内置 MCP server（stdio JSON-RPC），任何支持 Model Context Protocol 的 AI 客户端（Claude Desktop、Cursor 等）可直接创建 session、执行 JS、读取指纹：

```bash
# 启动 MCP server（stdio 模式，接入你的 AI 客户端配置）
./bes mcp
```

## 开发路线图

详见 [docs/roadmap.md](docs/roadmap.md)

## License

MIT
