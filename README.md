# browser-env-sandbox

> 基于 V8 引擎的本地 Chrome 沙箱环境 —— 让浏览器检测 JS 以为自己跑在真 Chrome 里，无需启动真实浏览器。

## 为什么需要这个项目？

在 JS 逆向（WAF 挑战、签名 SDK、风控指纹）中，很多目标代码会检测运行时环境是否为真实浏览器。Node.js 虽然也有 V8 引擎，但缺少 `window`、`navigator`、`document`、DOM 等浏览器 API，直接 `eval` 浏览器 JS 必崩。

传统做法是「补环境」——手动 mock 缺失的 API。但痛点在于：**每换一个目标 SDK（ →  → 

本项目把这些经验固化成一层稳定的环境快照 + vm 沙箱，**一次构建，到处复用**。

## 核心架构

```
┌─────────────────────────────────────────────────────┐
│                   browser-env-sandbox                │
│                                                      │
│  ┌─────────────┐  ┌──────────────┐  ┌─────────────┐ │
│  │  快照层      │  │  vm 沙箱层    │  │  网络转发层  │ │
│  │  Snapshot   │→ │  Sandbox     │→ │  NetBridge  │ │
│  │             │  │              │  │             │ │
│  │ CDP dump    │  │ vm.createCtx │  │ XHR/fetch   │ │
│  │ 真 Chrome   │  │ 快照灌入     │  │ → curl_cffi │ │
│  │ 全套属性    │  │ DOM mock     │  │ TLS 指纹    │ │
│  └─────────────┘  └──────────────┘  └─────────────┘ │
│                                                      │
│  目标 JS (WAF///
│  产出: 签名 / cookie / token                          │
└─────────────────────────────────────────────────────┘
```

- **环境快照层** — 用 CDP 从真 Chrome dump 全套 `navigator/screen/window/document` 属性，作为 ground truth
- **VM 沙箱层** — Node `vm.createContext` 创建隔离环境，快照数据 + DOM mock 拼成完整浏览器环境
- **网络转发层** — 沙箱内的 `XMLHttpRequest/fetch` 转发给 curl_cffi（带 TLS 指纹），响应灌回沙箱

## 快速开始

```bash
# 安装依赖
npm install

# dump 真浏览器环境快照（需要本地 Chrome + CDP）
node src/snapshot/cdp-dump.js --chrome-version 131

# 在沙箱中执行目标 JS
node src/index.js --snapshot snapshots/chrome-131.json --script target.js
```

## 设计约束（来自实战踩坑）

- `navigator` 属性必须和请求 UA 版本一致
- `document.URL`、`innerWidth/innerHeight` 必须可配置（参与签名计算）
- `top/parent/frames` 不能用 Proxy（直接崩溃）
- canary 探针（如 `navigator.pemrissions` 错拼）要正确返回 `undefined`
- 事件循环需模拟（`setTimeout/setInterval/requestAnimationFrame`）

详见 [docs/design-constraints.md](docs/design-constraints.md)

## 开发路线图

详见 [docs/roadmap.md](docs/roadmap.md)

## License

MIT
