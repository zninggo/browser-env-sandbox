# 架构设计文档

> browser-env-sandbox 核心架构 · v0.1
> 2026-08-27 起草

## 1. 项目定位

### 1.1 解决什么问题

JS 逆向（WAF 挑战页、签名 SDK、风控指纹）中，目标代码会检测运行时环境是否为真实浏览器。检测维度包括但不限于：

| 检测维度 | 典型检查 | Node.js 缺陷 |
|---------|---------|-------------|
| 全局对象 | `window`, `self`, `globalThis` 身份 | Node 的 `globalThis` ≠ 浏览器的 |
| Navigator | `userAgent`, `platform`, `language`, `plugins` | 完全缺失 |
| Screen | `width`, `height`, `colorDepth` | 完全缺失 |
| Document | `URL`, `cookie`, `createElement` | 完全缺失 |
| DOM API | `getElementById`, `querySelector` | 完全缺失 |
| 网络 API | `XMLHttpRequest`, `fetch` | Node 的 fetch 行为不同 |
| 定时器 | `requestAnimationFrame`, `queueMicrotask` | 部分缺失 |
| Canvas/WebGL | `toDataURL`, `getParameter` 指纹 | 完全缺失 |
| 错误拼写的属性 | `navigator.pemrissions` (canary) | 需正确返回 undefined |
| Proxy 检测 | `typeof Proxy`, `Symbol.toStringTag` | 需精确模拟 |

### 1.2 核心矛盾

Node.js 的 V8 引擎与 Chrome 的 V8 引擎是同一个引擎，但**宿主环境（embedding）完全不同**：

- Chrome V8 → 绑定了 Blink（渲染引擎）、BlinkBindings（DOM API）、网络栈、GPU 进程
- Node V8 → 绑定了 libuv、Node API（fs/http/crypto）、N-API

同一个 JS 引擎，不同的宿主。本项目的本质是：**在 Node 的 V8 上，手工重建 Chrome 的宿主环境**。

### 1.3 与传统补环境的区别

| | 传统补环境 | 本项目 |
|---|---------|-------|
| 环境来源 | 手写 mock，靠报错驱动 | CDP 从真 Chrome dump，ground truth |
| 一致性 | 容易出现 UA 版本不匹配 | 快照锁定特定 Chrome 版本 |
| 复用性 | 每个目标重写 | 通用沙箱，多目标复用 |
| 可维护性 | 散落在各个脚本 | 统一框架，模块化 |
| 网络层 | 各自实现 | 统一 curl_cffi 转发 |

## 2. 三层架构

### 2.1 环境快照层 (Snapshot Layer)

**职责：** 从真实 Chrome 浏览器中采集完整的运行时环境数据，作为沙箱的 ground truth。

**实现方式：** 通过 CDP (Chrome DevTools Protocol) 连接本地 Chrome，在页面上下文中执行采集脚本，dump 全套属性。

**采集内容：**

```
navigator/
├── userAgent, platform, language, languages
├── vendor, vendorSub, productSub
├── plugins (NamedNodeMap mock), mimeTypes
├── hardwareConcurrency, deviceMemory
├── maxTouchPoints, cookieEnabled, doNotTrack
├── onLine, battery, connection
├── permissions (含 canary: pemrissions → undefined)
├── webdriver (必须 false)
├── userAgentData (Chrome UA-CH)
├── mediaDevices, geolocation
├── serviceWorker, clipboard
├── storage, credentials
└── ... 所有可枚举 + 不可枚举属性

screen/
├── width, height, availWidth, availHeight
├── colorDepth, pixelDepth
├── orientation
└── availLeft, availTop

window/
├── innerWidth, innerHeight, outerWidth, outerHeight
├── devicePixelRatio, screenX, screenY
├── scrollX, scrollY, pageXOffset, pageYOffset
├── location (href, origin, protocol, host, pathname, search, hash)
├── history (length, state, scrollRestoration)
├── localStorage, sessionStorage (Storage mock)
├── indexedDB
├── performance (timing, memory, entries)
├── crypto (subtle, getRandomValues)
├── Intl (DateTimeFormat, Collator, NumberFormat)
└── ... 全套可枚举属性

document/
├── URL, documentURI, baseURI, referrer
├── cookie (可读写，需 mock CookieStore)
├── title, domain, origin
├── readyState, visibilityState
├── characterSet, contentType
├── createElement (返回最小 DOM Element mock)
├── getElementById, querySelector, querySelectorAll
├── head, body (最小 DOM 树)
└── ... 全套属性
```

**快照格式：** JSON 文件，按 Chrome 版本命名（如 `chrome-131.json`），包含：

```json
{
  "meta": {
    "chrome_version": "131.0.6778.87",
    "ua": "Mozilla/5.0 ...",
    "dumped_at": "2026-08-27T...",
    "cdp_url": "ws://127.0.0.1:9222/..."
  },
  "navigator": { ... },
  "screen": { ... },
  "window": { ... },
  "document": { ... },
  "globals": {
    "chrome": { "runtime": {} },
    "Buffer": null,
    "process": null
  }
}
```

**版本管理：** 每个快照锁定一个 Chrome 大版本。请求 UA 必须与快照版本一致，否则检测会不匹配。

### 2.2 VM 沙箱层 (Sandbox Layer)

**职责：** 在 Node 的 `vm.createContext` 中构建隔离的浏览器环境，灌入快照数据 + DOM mock，让目标 JS 在其中执行。

**核心设计：**

```
┌─────────────── vm.createContext ───────────────┐
│                                                 │
│  globalThis (= window = self = global)          │
│  ├── navigator   ← 快照数据 (冻结/只读)         │
│  ├── screen      ← 快照数据 (冻结/只读)         │
│  ├── document    ← 快照数据 + DOM mock (可写)   │
│  ├── location    ← 可配置 (参与签名)            │
│  ├── history     ← mock                         │
│  ├── localStorage / sessionStorage ← Storage mock│
│  ├── performance ← mock (timing + now)          │
│  ├── crypto      ← 原生 WebCrypto or polyfill   │
│  ├── Intl        ← 原生 (Node 已有)             │
│  ├── setTimeout / setInterval ← vm 内可控       │
│  ├── XMLHttpRequest ← → NetBridge               │
│  ├── fetch       ← → NetBridge                  │
│  ├── chrome      ← { runtime: {} } (Chrome 特征)│
│  ├── Buffer      ← undefined (抹去 Node 痕迹)   │
│  ├── process     ← undefined (抹去 Node 痕迹)   │
│  ├── require     ← undefined (抹去 Node 痕迹)   │
│  └── ...                                        │
│                                                 │
│  目标 JS 在此执行                                │
└─────────────────────────────────────────────────┘
```

**关键设计决策：**

1. **`window = globalThis = self`** — 三者指向同一对象，模拟浏览器全局对象身份
2. **navigator/screen 冻结** — `Object.freeze` 防止目标 JS 篡改后自我检测不一致
3. **document 可写** — 目标 JS 会写 cookie、创建元素，document 需要可变
4. **location 可配置** — `document.URL` 和 `location.href` 参与签名计算，必须可注入
5. **Node 痕迹抹除** — `Buffer`, `process`, `require`, `__dirname`, `module` 等设为 undefined
6. **Proxy 谨慎使用** — `top/parent/frames` 不包 Proxy（会崩溃），用普通对象 + getter
7. **canary 探针** — 故意拼错的属性（`navigator.pemrissions`）返回 undefined，不能抛错
8. **toString 伪装** — `navigator.toString()` 返回 `"[object Navigator]"`，函数的 `toString` 返回原生函数格式

**DOM Mock 策略：**

不做完整 DOM 实现（那是 jsdom 的领域），只实现逆向场景需要的最小子集：

- `document.createElement(tagName)` → 返回 Element mock（含 `style`, `setAttribute`, `getAttribute`, `appendChild`, `innerHTML`, `getContext`）
- `document.getElementById(id)` → 按 mock DOM 树查找
- `document.querySelector/querySelectorAll` → 最小 CSS 选择器解析
- `document.cookie` → CookieStore mock（get/set/toString）
- Canvas `getContext('2d')` / `getContext('webgl')` → 返回 mock context（`toDataURL` 返回固定/随机指纹）

**事件循环模拟：**

- `setTimeout/setInterval` — 使用 Node 原生（vm 上下文外），但回调在 vm 内执行
- `requestAnimationFrame` — 映射到 setTimeout(~16ms)
- `queueMicrotask` — 使用 `Promise.resolve().then()`
- `MutationObserver` — mock，回调可空操作或记录
- `MessageChannel/MessagePort` — mock

### 2.3 网络转发层 (Network Bridge Layer)

**职责：** 拦截沙箱内的 `XMLHttpRequest` 和 `fetch` 调用，转发给外部 HTTP 客户端（curl_cffi，带 TLS 指纹），将响应灌回沙箱。

**架构：**

```
沙箱内 JS 调用                  沙箱外 (Node 主线程)
─────────────                  ──────────────────
XMLHttpRequest.open()    →     NetBridge.request()
fetch()                  →     NetBridge.request()
                               │
                               ├─ curl_cffi (impersonate=chrome131)
                               │   ├─ TLS 指纹 (JA3/JA4)
                               │   ├─ HTTP/2 settings
                               │   └─ Header order
                               │
                               └─ 响应 → 灌回沙箱
                                   ├─ XHR: readyState 4, status, responseText
                                   └─ fetch: Response object mock
```

**关键设计：**

1. **XHR 状态机** — 完整模拟 `readyState` 0→1→2→3→4，触发 `onreadystatechange/onload/onerror`
2. **fetch Response mock** — 返回带有 `.json()/.text()/.arrayBuffer()` 的 Response 对象
3. **同步/异步** — XHR 的 `open(method, url, async=false)` 同步模式需特殊处理（vm 外阻塞）
4. **Cookie 透传** — 响应 Set-Cookie 自动写入 document.cookie 和快照的 cookie jar
5. **Header 顺序** — curl_cffi 的 header 顺序需与浏览器一致（部分 WAF 检测 header 顺序）
6. **重定向控制** — 可配置是否跟随重定向（WAF 挑战页常需手动处理 302）

**与 curl_cffi 的集成方式：**

- 方案 A：通过 Python 子进程调用 curl_cffi（`python3 -c "..."`），简单但开销大
- 方案 B：通过 HTTP 调用本地 curl_cffi 服务（起一个 Flask/FastAPI 微服务），开销小
- 方案 C：Node 原生库（如 `curl-impersonate` 的 Node binding），需调研

初期走方案 A（最快验证），稳定后迁移到方案 B。

## 3. 与逆向工作流的集成

参照 js-reverse skill 的五阶段工作流：

```
Observe → Capture → Rebuild → Patch → DeepDive
                              ↑         ↑
                    本项目覆盖区域
```

- **Rebuild 阶段**：从 Capture 阶段拿到的页面证据（脚本 URL、参数样例、调用顺序），在沙箱中重建执行环境
- **Patch 阶段**：按报错和 first divergence 驱动补环境，直到沙箱内稳定跑出目标参数
- **DeepDive 阶段**：沙箱跑通后，可在其中插桩、去混淆、提取算法

## 4. 模块依赖关系

```
index.js (入口)
  ├── snapshot/loader.js ──→ snapshots/chrome-XXX.json
  │
  ├── sandbox/context.js (vm.createContext 封装)
  │     ├── sandbox/browser-env.js (环境构建)
  │     │     ├── navigator ← 快照
  │     │     ├── screen ← 快照
  │     │     ├── document + dom-mock.js
  │     │     ├── location (可配置)
  │     │     ├── storage mock
  │     │     ├── performance mock
  │     │     └── timers (vm 内)
  │     │
  │     └── sandbox/network-bridge.js
  │           └── curl_cffi (Python subprocess / HTTP service)
  │
  └── utils/logger.js
```

## 5. 使用流程

```
1. dump 快照 (一次性)
   $ node src/snapshot/cdp-dump.js
   → snapshots/chrome-131.json

2. 配置环境
   const sandbox = createSandbox({
     snapshot: 'chrome-131',
     location: 'https://example.com/login',
     cookies: { ... }
   });

3. 执行目标 JS
   const result = sandbox.eval(targetJsCode);
   → { signature, cookies, tokens }

4. 提取产出
   sandbox.document.cookie  → 完整 cookie jar
   sandbox.window.__    → SDK 运行时状态
```

## 6. 扩展性设计

- **快照可插拔** — 不同 Chrome 版本、不同设备类型（PC/Mobile）的快照可热切换
- **DOM mock 可扩展** — 目标需要新的 DOM API 时，在 `dom-mock.js` 中增量添加
- **网络层可替换** — NetBridge 接口统一，底层可换 curl_cffi / got / axios
- **插件系统** — 预留 hook 机制，允许在 eval 前后注入自定义逻辑（如自动 hook 加密函数）
