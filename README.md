# browser-env-sandbox

> 基于 V8 引擎的浏览器环境模拟平台 —— 程序化生成真实指纹、高性能 VM 执行、反检测沙箱

## 这是什么

一个用 **Go + V8** 构建的浏览器环境模拟平台。不是补环境工具，是一个完整的反检测 VM 生态：

- **指纹引擎** — 程序化生成自洽的浏览器指纹（navigator/canvas/WebGL/Audio/字体/时区），随机但内部一致
- **V8 沙箱** — 纯 V8 Isolate 执行浏览器 JS，零 Node 痕迹，Isolate 池化高性能复用
- **网络层** — 离线 replay + 在线转发双模式，session-unique 防关联
- **调试层** — 暴露 CDP 协议，真 Chrome DevTools 可直连调试
- **多语言桥接** — JSON-over-HTTP 服务 + Python/Go/Node SDK + CLI

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

```bash
# 编译
go build -o bes ./cmd/bes

# 生成指纹
bes fingerprint --browser chrome --os windows --seed random

# 在沙箱中执行 JS
bes run --script target.js --fingerprint auto

# 启动 HTTP API 服务
bes-server --port 8080 --pool 8

# 离线 replay
bes replay --recording session.json --fingerprint auto
```

## Python SDK

```python
from bes import Sandbox

sandbox = Sandbox(fingerprint="auto")  # 随机自洽指纹
sandbox.eval("navigator.userAgent")
sandbox.load_script("target.js")
result = sandbox.call("sign", params)
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

每个会话独立：TLS 指纹 + cookie jar + 代理 IP + 指纹 + UA。多账号操作时各 session 之间零关联。

### 离线 Replay

录制真实浏览器请求序列 → 离线重放给沙箱。沙箱内的 XHR/fetch 不发真请求，而是从录制中取响应。用于反复调试签名算法。

## 开发路线图

详见 [docs/roadmap.md](docs/roadmap.md)

## License

MIT
