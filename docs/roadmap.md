# 开发路线图

## Phase 0: 架构确立 ✅ (2026-08-27)
- [x] 确定技术栈: Go + v8go
- [x] 源码经验整理到 design-constraints.md
- [x] ARCHITECTURE.md 编写
- [x] README.md 编写
- [x] go.mod 创建
- [ ] git commit

## Phase 1: 指纹引擎 (fpengine)
> 最核心也最难，决定项目上限

- [ ] 指纹知识库 (Chrome × Windows × 常见 GPU)
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

## Phase 3: sso WAF 验证场景
- [ ] 获取 sso WAF 挑战 JS
- [ ] 沙箱中 eval 挑战 JS
- [ ] 报错 → 补环境迭代
- [ ] 
- [ ] 签名与真实浏览器对比
- [ ] 验证报告

里程碑 M3: sso WAF 签名一致 — 第一个实战验证

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
- [ ] protobuf 定义 (sandbox.proto)
- [ ] gRPC 服务器
- [ ] 会话管理 (多 session 并发)
- [ ] Python SDK
- [ ] Go SDK
- [ ] Node SDK
- [ ] CLI (bes)
- [ ] **验证: Python SDK 调用沙箱跑通签名**

里程碑 M6: 桥接层可用 — 多语言 SDK 可用

## Phase 7: Session-Unique + 通用化
- [ ] Session-unique 完整实现 (TLS+cookie+proxy+指纹)
- [ ] 多 session 并发隔离验证
- [ ] 快照/指纹热切换
- [ ] 设备类型 (PC/Mobile)
- [ ] 其他目标 SDK 验证 (, 其他 WAF)
- [ ] 性能基准 (Isolate 池化 vs 新建)
- [ ] 文档完善

里程碑 M7: 通用平台 — 多账号反检测可用

## 里程碑总览

| 里程碑 | 内容 | 产出 |
|-------|------|------|
| M1 | 指纹引擎 | 自洽指纹通过检测 |
| M2 | V8 沙箱 | 浏览器环境检查通过 |
| M3 | sso 验证 | 
| M4 | 网络层 | replay + 转发双模式 |
| M5 | 调试层 | DevTools 可调试 |
| M6 | 桥接层 | 多语言 SDK |
| M7 | 通用化 | 多账号反检测 |
