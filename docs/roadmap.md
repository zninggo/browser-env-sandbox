# 开发路线图

> 最后核查: 2026-08-29 (按代码实际状态回填勾选)

## Phase 0: 架构确立 ✅ (2026-08-27)
- [x] 确定技术栈: Go + v8go
- [x] 设计约束整理到 design-constraints.md
- [x] ARCHITECTURE.md 编写
- [x] README.md 编写
- [x] go.mod 创建
- [x] git commit

## Phase 1: 指纹引擎 (fpengine) ✅
> 最核心也最难，决定项目上限

- [x] 指纹知识库（Chrome × Windows × 常见 GPU）
- [x] 指纹生成器 (种子 → 自洽指纹)
- [x] navigator 属性合成
- [x] screen 属性合成
- [x] canvas 指纹合成
- [x] WebGL 指纹合成
- [x] AudioContext 指纹合成
- [x] 字体集合成
- [x] 时区 + 语言合成
- [x] 自洽性校验器
- [x] **验证：生成的指纹通过 CreepJS / BrowserLeaks 检测**

里程碑 M1: 指纹引擎可用 — 生成的指纹自洽且通过检测 ✅

## Phase 2: V8 沙箱引擎 (sandbox) ✅
- [x] v8go 基础封装 (Isolate + Context)
- [x] Isolate 池化
- [x] 指纹灌入 (Go → V8 Object)
- [x] 浏览器 API mock (navigator/screen/document/window)
- [x] DOM 最小子集 (createElement/getElementById/cookie)
- [x] location 可配置
- [x] Storage mock (localStorage/sessionStorage)
- [x] 定时器 (setTimeout/setInterval/requestAnimationFrame)
- [x] console mock
- [x] 事件循环模拟 + flushTimers
- [x] **验证：沙箱内执行基本浏览器检测 JS 无报错**

里程碑 M2: 沙箱可用 — 基本浏览器环境检查通过 (221/221 ✅)

## Phase 3: 端到端验证场景
- [ ] 获取典型页面 JS（含环境探测逻辑）
- [ ] 沙箱中 eval 该 JS
- [ ] 报错 → 补环境迭代
- [ ] 输出与真实浏览器对比
- [ ] 验证报告

里程碑 M3: 环境兼容性验证 — 典型页面 JS 在沙箱中与真实浏览器行为一致

## Phase 4: 网络层 (netlayer)
- [x] XHR/fetch 拦截架构设计
- [x] 离线 replay 引擎
- [x] 在线转发（纯 Go utls，curl-impersonate/curl_cffi 死代码已移除）
- [x] Cookie jar (与 document.cookie 联动)
- [x] TLS 指纹配置（utls 预设按会话指纹 Chrome 版本动态选择，PQ sig algs 仅 ≥140 注入；tlsTarget per-session 透传）
- [x] 代理管理 (per-session)
- [x] 请求录制
- [x] **XHR/fetch → Go FunctionCallback → NetHandler 接线** (2026-08-28)
- [x] **Cookie 自动同步 (Set-Cookie → CookieStore → document.cookie)** (2026-08-28)
- [x] **验证：沙箱内 XHR → 真实 HTTP 请求 → 响应回传完整链路** (2026-08-28)
- [x] 原生 Go TLS 指纹（不依赖 curl_cffi Python 子进程）
- [x] 同步 + 异步 XHR 双模式（`__besNetRequest` 同步阻塞 / `__besNetRequestAsync` + setTimeout 回 Isolate 线程；Bug 6 恢复同步 XHR，回调捕获已在 Isolate 线程完成）
- [x] H2/UA 版本对齐（删硬编码默认 UA，UA 一律由会话指纹提供）(2026-08-29)

里程碑 M4: 网络层可用 — 沙箱 XHR/fetch 可发真实网络请求 ✅ (2026-08-28)

## Phase 5: 调试层 (debug)
- [x] CDP WebSocket 服务器
- [x] Runtime 域 (evaluate, callFunctionOn)
- [x] Network 域 (请求采集, 与 netlayer 联动)
- [x] Debugger 域 (断点, 步进)
- [x] Console 域 (消息转发)
- [x] **验证: chrome://inspect 连接沙箱, DevTools 可用**

里程碑 M5: 调试层可用 — 真 Chrome DevTools 可调试沙箱 ✅

## Phase 6: 桥接层 (bridge) + SDK ✅
- [x] JSON-over-HTTP API (Go 1.22 ServeMux)
- [x] 会话管理 (多 session 并发)
- [x] Python SDK
- [x] Go SDK
- [x] Node SDK
- [x] CLI (bes)
- [x] **验证: Python SDK 调用沙箱执行 JS**

里程碑 M6: 桥接层可用 — 多语言 SDK 可用 ✅

## Phase 7: Session-Unique + 通用化
- [x] Session-unique 完整实现 (TLS+cookie+proxy+指纹)
- [x] 多 session 并发隔离验证
- [x] 快照/指纹热切换
- [x] 设备类型 (PC/Mobile)
- [x] **Profile 持久化接线（save-profile / resume API，指纹 seed + cookie 还原）** (2026-08-29)
- [x] **idle session 自动清理（30 分钟，bridge Service 内置）** (2026-08-29)
- [ ] 其他环境验证（更多页面 JS 场景）
- [x] 性能基准 (Isolate 池化 vs 新建)
- [ ] 文档完善

里程碑 M7: 通用平台 — 多场景隔离可用 ✅

## Phase 8: 功能补全 (2026-08-27 调研后)

### P0 — 基础设施补全
- [x] Dockerfile + docker-compose 部署
- [x] MCP (Model Context Protocol) server 适配
- [x] 原生 Go TLS 指纹（utls + post-quantum signature schemes，不再依赖 curl_cffi/curl-impersonate；ClientHello 预设按会话指纹版本动态选择）
- [ ] HTTP/3 (QUIC) 支持（假实现空壳已删除；真实需求出现时接 quic-go）
- [x] Playwright/Puppeteer 兼容层

### P1 — 功能增强
- [x] WebGPU 指纹
- [x] Client Hints 完整 (Sec-CH-UA-Full-Version-List)
- [x] 行为生物特征模拟 (鼠标轨迹/键盘节奏)
- [x] Profile 持久化 + 导入/导出/分享
- [x] 完整 CSS 选择器引擎
- [x] HTML 解析器 (DOMParser 返回真 DOM 树)
- [x] GUI/Dashboard (Web 管理界面)
- [x] 代理健康检查 + 自动切换

### P2 — 优化
- [x] 升级 V8 版本 (v8go V8 9.0 → deno_core 或新版 v8go)
- [x] 性能基准
- [x] 验证码识别集成

## 里程碑总览

| 里程碑 | 内容 | 产出 |
|-------|------|------|
| M1 | 指纹引擎 | 自洽指纹通过检测 ✅ |
| M2 | V8 沙箱 | 浏览器环境检查通过 (221/221 ✅) |
| M3 | 端到端验证 | 典型页面 JS 行为一致 |
| M4 | 网络层 | replay + 转发双模式 ✅ |
| M5 | 调试层 | DevTools 可调试 ✅ |
| M6 | 桥接层 | 多语言 SDK ✅ |
| M7 | 通用化 | 多场景隔离 ✅ |
| M8 | 功能补全 | Docker + MCP + Playwright 兼容 ✅ |

## 第二轮残留 Bug 修复 (2026-08-29)

- [x] Bug A: asyncCallback goroutine 内 `AsFunction()`/`Global()` 调用移回 Isolate 线程（并发 50 次 async fetch 压测无崩溃）
- [x] Bug B: measureText 非线性宽度（per-character advance 表 + per-fingerprint 抖动，返回真实 TextMetrics 对象）
- [x] Bug C: crypto.getRandomValues 改用 Go `crypto/rand`（`__besRandomBytes` base64 过桥，65536 上限符合规范）
- [x] Bug D: URL 解析支持 IPv6 `[::1]:8080`、`user:pass@` 凭据、默认端口省略、WHATWG origin
- [x] Bug E: `iframe.contentWindow !== window`（`Object.create(window)` 隔离，contentDocument 同理）
- [x] Bug F: `new Event()` / `new CustomEvent()` 补 preventDefault/stopPropagation/stopImmediatePropagation/initEvent（JS BesEvent 包装 + Event.prototype 方法）
- [x] Bug G: BES-USAGE.md 端口 18900 → 19821（与 SDK/Docker 默认一致）
- [x] Bug H: roadmap.md 同步/异步 XHR 描述更新 + Node SDK 提供 `Sandbox.create()` 静态工厂与 `ready` promise

## TLS 版本一致性 + 死代码清理 + Profile 接线 (2026-08-29)

- [x] utls ClientHello 预设按会话指纹 Chrome 版本动态选择（`utlsPresetFor`：≥133 → HelloChrome_133，120-132 → HelloChrome_131）
- [x] PQ signature algorithms（0x0904-0x0906）仅 Chrome ≥140 条件注入
- [x] H2/HTTP1.1 硬编码默认 UA（Chrome/131）删除，UA 一律由会话指纹提供
- [x] tlsTarget per-session 透传（NetHandlerFactory 带 fingerprint，bes-server 用 `fp.Browser.Version` 构建 target）
- [x] 死代码删除：`curl_impersonate.go`（CurlImpersonate/CurlCffiClient）、`tls_profile.go`（DefaultTLSProfiles/GetTLSProfile）、`buildChromeClientHelloSpec`、`quic.go`（假 H3）；活代码重组为 `tls_client.go` + `proxy_pool.go`
- [x] internal/session 处置：删除与 bridge Service 重复的 Manager，ProfileStore 接线 bridge（save-profile/resume API + idle 清理移植）
- [x] 验证：WSL CGO 编译通过 + selftest 210/210 + profile API 冒烟（快照→恢复→cookie/UA 还原全链路）

## Worker + ES Module + console 修复 (2026-08-29)

- [x] Web Worker 真实执行：独立 Isolate + inbound/outbound channel 双泵（`worker.go` StartWorker + startParentPump，约束 11 线程模型）
- [x] ES Module：`importModule()` polyfill，支持 `data:`/`blob:` URL，`export default` + named exports
- [x] console.log 修复：v8go v0.34.0 ObjectTemplate 嵌套 FunctionTemplate Go callback 永不触发——改用 PostContext `GetFunction` + `global.Set`（`injectConsole`）
- [x] Worker UA 注入 + blob URL 支持 + CLI 消息 drain grace period

## WebSocket + Canvas 回放 + 联合分布 + OOM 防护 (2026-08-29)

- [x] WebSocket 真实连接：纯 Go RFC 6455 实现（握手 + 帧编解码 + mask），`wsBridge` 线程模型参照 Worker（read/write goroutine + `scheduleTimer` 回 Isolate 线程），JS 侧标准接口（onopen/onmessage/onclose/onerror + send/close + readyState 常量）
- [x] Canvas 预采集回放：`canvas_dataset.go` 懒加载 `data/canvas_dataset.json`，`toDataURL()` 按指纹组合查表返回真实值，未命中降级合成；`CanvasFP.ToDataURL` 字段
- [x] 指纹采样联合分布：`filterGPUsByOS` + `filterScreensByOS` 预过滤（macOS→Metal/Retina，Windows→Direct3D11/24-32bit，Linux→no D3D/Metal），100 次采样合理性断言
- [x] V8 堆内存上限：`WithResourceConstraints(8MB initial, 512MB max)`，`BES_POOL_MEM_MB` 可配置；OOM 脚本优雅终止（ExecutionTerminated）而非进程崩溃
- [x] 验证：WSL CGO 编译通过 + selftest 221/221
