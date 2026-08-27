# 开发路线图

## Phase 0: 项目骨架 ✅ (2026-08-27)
- [x] 项目目录结构
- [x] README.md
- [x] ARCHITECTURE.md
- [x] 设计约束文档
- [x] package.json
- [ ] git init + 首次 commit
- [ ] GitHub 仓库创建

## Phase 1: CDP 环境快照工具
- [ ] CDP 连接模块（连接本地 Chrome DevTools）
- [ ] navigator 全属性采集脚本
- [ ] screen 全属性采集脚本
- [ ] window 全属性采集脚本
- [ ] document 全属性采集脚本
- [ ] chrome 对象采集
- [ ] 快照 JSON 格式化 + 版本标注
- [ ] 验证：dump chrome-131 快照，对比真实属性

## Phase 2: 最小 VM 沙箱
- [ ] vm.createContext 封装（context.js）
- [ ] 快照加载器（loader.js）
- [ ] navigator/screen 灌入（冻结）
- [ ] document mock + DOM 最小子集
- [ ] location 可配置注入
- [ ] localStorage/sessionStorage mock
- [ ] Node 痕迹抹除
- [ ] toString/Symbol.toStringTag 伪装
- [ ] 验证：在沙箱中执行 `navigator.userAgent`、`document.cookie` 等基本检查

## Phase 3: sso WAF 挑战 JS 验证（第一个验证场景）
- [ ] 获取 sso WAF 挑战页 JS（
- [ ] 在沙箱中 eval 挑战 JS
- [ ] 记录报错 → 补环境迭代
- [ ] 验证：沙箱中跑通 
- [ ] 验证：签名与真实浏览器一致
- [ ] 产出：experiments/sso-waf-challenge/ 验证报告

## Phase 4: 网络转发层
- [ ] XMLHttpRequest 状态机 mock
- [ ] fetch Response mock
- [ ] curl_cffi 集成（Python subprocess）
- [ ] Cookie 透传（Set-Cookie → document.cookie）
- [ ] Header 顺序保持
- [ ] 验证：沙箱内 XHR 请求 → curl_cffi 转发 → 响应灌回

## Phase 5: 通用化 + 文档
- [ ] 快照热切换（不同 Chrome 版本）
- [ ] 设备类型支持（PC/Mobile navigator 切换）
- [ ] 插件/hook 系统
- [ ] curl_cffi 服务化（方案 B）
- [ ] API 文档
- [ ] 使用示例（ /  / 
- [ ] 性能基准

## 里程碑

| 里程碑 | 目标 | 预期产出 |
|-------|------|---------|
| M1 | 快照工具可用 | chrome-131.json 完整快照 |
| M2 | 沙箱可执行基本 JS | navigator/screen/document 检查通过 |
| M3 | sso WAF 签名跑通 | 
| M4 | 网络层可用 | 沙箱内 XHR 完整请求-响应循环 |
| M5 | 通用化完成 | 多目标 SDK 复用验证 |
