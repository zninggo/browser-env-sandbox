# 开发路线图

## Phase 0: 架构确立 ✅ (2026-08-27)
- [x] 确定技术栈: Go + v8go
- [x] 设计约束整理到 design-constraints.md
- [x] ARCHITECTURE.md 编写
- [x] README.md 编写
- [x] go.mod 创建
- [x] git commit

## Phase 1: 指纹引擎 (fpengine)
> 最核心也最难，决定项目上限

- [ ] 指纹知识库（Chrome × Windows × 常见 GPU）
- [ ] 指纹生成器 (种子 → 自洽指纹)
- [ ] navigator 属性合成
- [ ] screen 属性合成
- [ ] canvas 指纹合成
- [ ] WebGL 指纹合成
- [ ] AudioContext 指纹合成
- [ ] 字体集合成
- [ ] 时区 + 语言合成
- [ ] 自洽性校验器
- [ ] **验证：生成的指纹通过 CreepJS / BrowserLeaks 检测**

里程碑 M1: 指纹引擎可用 — 生成的指纹自洽且通过检测

## Phase 2: V8 沙箱引擎 (sandbox)
- [ ] v8go 基础封装 (Isolate + Context)
- [ ] Isolate 池化
- [ ] 指纹灌入 (Go → V8 Object)
- [ ] 浏览器 API mock (navigator/screen/document/window)
- [ ] DOM 最小子集 (createElement/getElementById/cookie)
- [ ] location 可配置
- [ ] Storage mock (localStorage/sessionStorage)
- [ ] 定时器 (setTimeout/setInterval/requestAnimationFrame)
- [ ] console mock
- [ ] 事件循环模拟 + flushTimers
- [ ] **验证：沙箱内执行基本浏览器检测 JS 无报错**

里程碑 M2: 沙箱可用 — 基本浏览器环境检查通过

## Phase 3: WAF 验证场景
- [ ] 获取目标 WAF 挑战 JS
- [ ] 沙箱中 eval 挑战 JS
- [ ] 报错 → 补环境迭代
- [ ] 签名与真实浏览器对比
- [ ] 验证报告

里程碑 M3: WAF 签名一致 — 第一个实战验证

## Phase 4: 网络层 (netlayer)
- [ ] XHR/fetch 拦截 (Go 侧)
- [ ] 离线 replay 引擎
- [ ] 在线转发 (curl-impersonate 集成)
- [ ] Cookie jar (与 document.cookie 联动)
- [ ] TLS 指纹配置 (与 UA 版本一致)
- [ ] 代理管理 (per-session)
- [ ] 请求录制
- [ ] **验证：沙箱内 XHR → replay 响应 → 签名计算完整链路**

里程碑 M4: 网络层可用 — 离线 replay + 在线转发双模式

## Phase 5: 调试层 (debug)
- [ ] CDP WebSocket 服务器
- [ ] Runtime 域 (evaluate, callFunctionOn)
- [ ] Network 域 (请求采集, 与 netlayer 联动)
- [ ] Debugger 域 (断点, 步进)
- [ ] Console 域 (消息转发)
- [ ] **验证: chrome://inspect 连接沙箱, DevTools 可用**

里程碑 M5: 调试层可用 — 真 Chrome DevTools 可调试沙箱

## Phase 6: 桥接层 (bridge) + SDK
- [x] JSON-over-HTTP API (Go 1.22 ServeMux)
- [x] 会话管理 (多 session 并发)
- [x] Python SDK
- [x] Go SDK
- [x] Node SDK
- [x] CLI (bes)
- [x] **验证: Python SDK 调用沙箱执行 JS**

里程碑 M6: 桥接层可用 — 多语言 SDK 可用

## Phase 7: Session-Unique + 通用化
- [ ] Session-unique 完整实现 (TLS+cookie+proxy+指纹)
- [ ] 多 session 并发隔离验证
- [ ] 快照/指纹热切换
- [ ] 设备类型 (PC/Mobile)
- [ ] 其他目标 SDK 验证 (其他 WAF)
- [ ] 性能基准 (Isolate 池化 vs 新建)
- [ ] 文档完善

里程碑 M7: 通用平台 — 多账号反检测可用

## Phase 8: 功能补全 (2026-08-27 调研后)

### P0 — 基础设施补全
- [x] Dockerfile + docker-compose 部署
- [x] MCP (Model Context Protocol) server 适配
- [ ] 原生 Go TLS 指纹（不依赖 curl-impersonate 二进制）
- [ ] HTTP/3 (QUIC) 支持
- [x] Playwright/Puppeteer 兼容层

### P1 — 功能增强
- [x] WebGPU 指纹
- [x] Client Hints 完整 (Sec-CH-UA-Full-Version-List)
- [ ] 行为生物特征模拟 (鼠标轨迹/键盘节奏)
- [x] Profile 持久化 + 导入/导出/分享
- [x] 完整 CSS 选择器引擎
- [ ] HTML 解析器 (DOMParser 返回真 DOM 树)
- [x] GUI/Dashboard (Web 管理界面)
- [x] 代理健康检查 + 自动切换

### P2 — 优化
- [ ] 升级 V8 版本 (v8go V8 9.0 → deno_core 或新版 v8go)
- [ ] 性能基准
- [ ] 验证码识别集成

## 里程碑总览

| 里程碑 | 内容 | 产出 |
|-------|------|------|
| M1 | 指纹引擎 | 自洽指纹通过检测 |
| M2 | V8 沙箱 | 浏览器环境检查通过 (133/133 ✅) |
| M3 | WAF 验证 | 目标 WAF 签名一致 |
| M4 | 网络层 | replay + 转发双模式 ✅ |
| M5 | 调试层 | DevTools 可调试 |
| M6 | 桥接层 | 多语言 SDK ✅ |
| M7 | 通用化 | 多账号反检测 |
| M8 | 功能补全 | Docker + MCP + Playwright 兼容 |
