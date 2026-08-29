# 公开开源方案调研与技术评估报告

> 调研日期：2026-08-29
> 数据来源：`gh api` 精确抓取 28 个 GitHub 仓库 + bes 源码实读 + WSL 实际运行验证
> 覆盖范围：四维度开源方案对标、代码实际状态审计、修补升级方案、底层基座决策、纯 V8 能力边界、逆向纯协议可行性

---

## 目录

1. [开源方案全景对标](#1-开源方案全景对标)
2. [bes 代码实际状态审计](#2-bes-代码实际状态审计)
3. [修补升级方案](#3-修补升级方案)
4. [底层基座：切换/并存/保持决策](#4-底层基座切换并存保持决策)
5. [纯 V8 环境能力边界](#5-纯-v8-环境能力边界)
6. [逆向纯协议可行性分析](#6-逆向纯协议可行性分析)
7. [渲染指纹补充方案（不引入浏览器）](#7-渲染指纹补充方案不引入浏览器)
8. [架构模式：通用引擎 + 外部补环境分离](#8-架构模式通用引擎--外部补环境分离)
9. [行动项总表](#9-行动项总表)

---

## 1. 开源方案全景对标

### 1.1 指纹生成/检测

| 项目 | GitHub | Stars | 最近更新 | 语言 | License | 定位 |
|------|--------|-------|---------|------|---------|------|
| FingerprintJS | fingerprintjs/fingerprintjs | 28,368 | 2026-08-09 | TS | MIT | 指纹**检测**（bes 验证基准） |
| apify/fingerprint-suite | apify/fingerprint-suite | 2,582 | 2026-08-20 | TS | Apache-2.0 | 指纹**生成**+注入 |
| browserforge | daijro/browserforge | 1,234 | 2026-02-26 | Python | Apache-2.0 | 指纹**生成**（bes 直接竞品） |
| CreepJS | abrahamjuliot/creepjs | 2,489 | 2026-06-11 | JS | — | 指纹**深度检测** |

**关键区分**：检测库（FingerprintJS/CreepJS）读取信号算哈希，是 bes 的验证基准；生成库（browserforge/apify）是 bes 的直接竞品。

**bes vs browserforge**：

| | browserforge | bes |
|---|---|---|
| 形态 | Python 配置生成器（输出 JSON） | Go+V8 执行引擎（真实运行 JS 验证） |
| 落地 | 必须注入真浏览器才生效 | 自身即沙箱，无宿主依赖 |
| 自洽 | 静态派生一致 | 显式约束传播 + 运行时校验两层 |
| 数据源 | Apify 真实流量 Bayesian 采样 | 手动知识库 + apify GPU/screen 数据 |
| 内容型维度 | 无 canvas/WebAudio/WebGPU | 全覆盖 |

**可借鉴**：apify `datapoints` 联合分布采样、Bayesian 频率建模、FingerprintJS 嵌入 selftest 作检测闭环。

### 1.2 JS 沙箱/补环境

| 项目 | GitHub | Stars | 最近更新 | 引擎类型 | 对 bes 价值 |
|------|--------|-------|---------|---------|------------|
| jsdom | jsdom/jsdom | 21,663 | 2026-08-26 | 纯 JS 模拟 | DOM 行为参照基线 |
| goja | dop251/goja | 7,059 | 2026-08-26 | 纯 Go（非 V8） | 无 CGO 备选引擎 |
| happy-dom | capricorn86/happy-dom | 4,619 | 2026-08-28 | 纯 JS 模拟 | 现代 DOM 参照 |
| rogchap/v8go | rogchap/v8go | 3,498 | **2024-08-02（停更）** | 真 V8 | bes 原始依赖（已弃用） |
| isolated-vm | laverdet/isolated-vm | 2,909 | 2026-08-23 | 真 V8（Node） | inspector 参照 |
| tommie/v8go | tommie/v8go | 149 | 2026-01-24 | 真 V8 | **bes 当前使用（已自行 fork+build）** |
| goja_nodejs | dop251/goja_nodejs | 438 | 2026-02-12 | 纯 Go | eventloop 参照 |
| quickjs-go | quickjs-go/quickjs-go | 103 | **2023-04-14（停更）** | QuickJS | 不推荐 |

**引擎选型结论**：v8go（真 V8）仍是正确选择——指纹/补环境的产出给浏览器风控看，V8 的 ECMAScript 语义与真实 Chrome 一致是核心资产。rogchap/v8go 已停更，bes 已迁移到 tommie fork 并自行 fork+build。

**可借鉴**：isolated-vm 的 v8 inspector 适配方式（补 CDP 断点）、jsdom DOM 行为测试用例、isolated-vm `memoryLimit`（OOM 防护）。

### 1.3 TLS 指纹模拟

| 项目 | GitHub | Stars | 最近更新 | 纯Go | 底座 | 定位 |
|------|--------|-------|---------|------|------|------|
| curl-impersonate | lwthiker/curl-impersonate | 6,906 | **2024-07-18（停更）** | ❌ C | — | bes 死代码依赖 |
| curl_cffi | lexiforest/curl_cffi | 6,408 | 2026-08-28 | ❌ Python | — | bes 死代码依赖 |
| **utls** | refraction-networking/utls | 2,544 | 2026-08-02 | ✅ | — | **bes 当前底座** |
| tls-client | bogdanfinn/tls-client | 1,814 | 2026-08-21 | ✅ | utls+fhttp | 最全纯 Go 方案 |
| CycleTLS | Danny-Dasilva/CycleTLS | 1,517 | 2026-07-07 | ✅ | utls+fhttp | Go+Node 方案 |
| azuretls-client | Noooste/azuretls-client | 471 | 2026-04-17 | ✅ | utls+BoringSSL | 会话式方案 |

**关键发现**：tls-client/CycleTLS/azuretls 底层全是 utls——与 bes 同根。bes 的 H2 帧级指纹（手动构造每一帧字节）比竞品的 fhttp 黑盒封装更透明可控。

### 1.4 反检测/无头自动化

| 项目 | GitHub | Stars | 最近更新 | 路线 | 定位 |
|------|--------|-------|---------|------|------|
| Puppeteer | puppeteer/puppeteer | 95,515 | 2026-08-28 | 真浏览器自动化 | Chrome/CDP 官方级 |
| Playwright | microsoft/playwright | 95,314 | 2026-08-29 | 真浏览器自动化 | 跨浏览器 E2E 标准 |
| SeleniumBase | seleniumbase/SeleniumBase | 12,968 | 2026-08-26 | 真浏览器+UC mode | 反检测测试框架 |
| undetected-chromedriver | ultrafunkamsterdam/undetected-chromedriver | 12,810 | 2025-07-05 | patch chromedriver | 反检测 Selenium |
| camoufox | daijro/camoufox | 11,513 | 2026-08-26 | 改造 Firefox | 反指纹浏览器 |
| puppeteer-extra | berstend/puppeteer-extra | 7,396 | **2024-07-18（停更）** | stealth 插件 | Puppeteer 反检测插件 |
| botasaurus | omkarcloud/botasaurus | 5,693 | 2026-07-26 | 组合方案 | 反检测爬虫框架 |
| nodriver | ultrafunkamsterdam/nodriver | 4,706 | 2026-05-13 | 免驱动直连 CDP | 去 webdriver 痕迹 |
| patchright | Kaliiiiiiiiii-Vinyzu/patchright | 4,208 | 2026-08-19 | Playwright patch | 堵 CDP 泄漏 |
| rebrowser-patches | rebrowser/rebrowser-patches | 1,423 | 2025-05-09 | 源码 patch | 堵 Runtime.Enable 泄漏 |

**bes 路线坐标**（从轻到重）：

```
bes(纯V8补环境) → puppeteer-stealth(JS补丁) → nodriver/rebrowser/patchright(驱动patch) → camoufox(改造浏览器) → Playwright/Puppeteer(无反检测)
   最轻 ◀─────────────────────────────────────────────────────────────────────────────▶ 最重
```

bes 与全部竞品的根本差异：上述项目都在"真浏览器怎么藏"的层次竞争，bes 直接不碰真浏览器。

---

## 2. bes 代码实际状态审计

### 2.1 AGENTS.md 与实际代码的偏差

| AGENTS.md 声称 | 实际代码 | 状态 |
|---------------|---------|------|
| v8go 用 `rogchap/v8go v0.9.0` | `go.mod` 锁 `tommie/v8go v0.34.0`（已迁移+自行 fork build） | ✅ 已修复 |
| TLS 三级降级链 curl-impersonate→curl_cffi→Go HTTP | `TLSClient` 只用 utls，降级链已移除 | ✅ 已修复（死代码残留） |
| 除 v8go 无第三方 Go 库，网络层全标准库 | 实际依赖 `utls v1.8.2` + `brotli` + `golang.org/x/net` | ⚠️ 过时 |
| Chrome 124-152 TLS 指纹 | utls 预设固定 `HelloChrome_133` + 手动 PQ 补丁 | ⚠️ 需修复 |
| HTTP/3 (QUIC) 支持 | `quic.go` `CheckAvailable()` 硬编码 `false`，空壳 | ⚠️ 未实现 |

### 2.2 TLS 实现实际状态

**已实现（比多数竞品精细）**：
- 纯 Go utls（无 Python 子进程、无 CGO），`HelloChrome_133` 预设
- 手动注入 post-quantum signature algorithms（0x0904-0x0906），模拟 Chrome 140+
- **手动构造 HTTP/2 每一帧字节**（`h2client.go`），从真实 Chrome 151 采集 akamai fingerprint
- 帧合并写入单个 TLS record（匹配 Chrome 行为）
- HTTP/1.1 回退（仅 ALPN 未协商 h2 时）
- Cookie jar（RFC 6265 scope）+ 代理 CONNECT

**问题清单**（2026-08-29 已修复 1/2/3/5/6，4 随死代码一并删除，7 保持现状）：

| # | 问题 | 严重度 | 状态 |
|---|------|--------|------|
| 1 | TLS 三层版本号互相不一致 | 🔴 高 | ✅ 已修复：per-session 版本透传 + utls 预设动态选择 |
| 2 | utls 预设固定 Chrome133，不随指纹版本切换 | 🔴 高 | ✅ 已修复：`utlsPresetFor` 版本映射 |
| 3 | `tls_profile.go` 所有版本数据完全相同 | 🟡 中 | ✅ 已删除（产出数据无消费者，死代码） |
| 4 | `tls_profile.go` 产出数据只存不读 | 🟡 中 | ✅ 已删除 |
| 5 | `tls_spec.go` `buildChromeClientHelloSpec()` 是死代码 | 🟡 中 | ✅ 已删除（PQ 常量保留） |
| 6 | `curl_impersonate.go` CurlImpersonate/CurlCffiClient 是死代码 | 🟡 中 | ✅ 已删除（TLSClient 移至 tls_client.go） |
| 7 | `quic.go` QUICClient 是空壳 | 🟡 中 | 保持现状（假实现误导性已消除，真实需求出现时接 quic-go） |

### 2.3 go.mod 实际依赖

```
github.com/andybalholm/brotli v1.0.6          // Brotli 解压
github.com/refraction-networking/utls v1.8.2   // TLS 指纹
github.com/tommie/v8go v0.34.0                 // V8 引擎（自行 fork+build）
golang.org/x/net v0.58.0                       // HTTP/2 帧
```

### 2.4 指纹引擎实际状态

**知识库覆盖**（`knowledge_base.go`）：
- Chrome 148-152（5 个版本）
- Windows 11 / macOS 14 / Linux（3 个 OS）
- GPU：Windows 6 款 + macOS 3 款 + Linux 3 款（另有 apify 真实数据 773 combos）
- Screen：每 OS 2-4 款（另有 apify 真实数据 4569 configs）
- 时区：6 个
- Canvas/Audio：合成生成（`CanvasHashes`/`AudioHashes` map 为空，用确定性种子生成）

**采样方式**：各维度独立 `rng.Intn` 均匀采样——非联合分布/频率加权。GPU 和 Screen 已接入 apify 真实数据但独立采样。

### 2.5 selftest 实际状态

133 项检查，全部是 API 存在性检查（`typeof X === "function"`），无外部检测库闭环（无 FingerprintJS/CreepJS）。

---

## 3. 修补升级方案

### Phase 1：TLS 版本一致性修复 + 死代码清理（P0/P1）

#### 1.1 utls 预设按指纹版本动态切换

**问题**：`utls_client.go:63` 固定 `HelloChrome_133`。

**改法**：
- `UTLSClient` 增加 `chromeVersion int` 字段
- `NewUTLSClient(target)` 解析版本号，映射到 utls 预设
- PQ sig algs 注入改为条件注入（仅 Chrome ≥140）
- `session/manager.go:78` 把版本号传给 `NewTLSClient`

#### 1.2 H2/UA 版本对齐

**问题**：`h2client.go:29` H2 帧从 Chrome 151 采集，`h2client.go:379` 默认 UA 硬编码 Chrome 131。

**改法**：删掉硬编码默认 UA，从指纹 UA 取值。

#### 1.3 清理死代码

| 文件 | 删除 | 保留 |
|------|------|------|
| `curl_impersonate.go` | `CurlImpersonate` + `CurlCffiClient`（~170 行） | `TLSClient`/`NewTLSClient` 等活代码（移到 `tls_client.go`） |
| `tls_spec.go` | `buildChromeClientHelloSpec()` | PQ sig algs 常量 |
| `tls_profile.go` | `DefaultTLSProfiles`/`GetTLSProfile`（路 A） | `TLSProfile` struct、`ProxyConfig`、`ProxyPool` |
| `quic.go` | 整个文件（空壳） | — |

#### Phase 1 验证标准
- `go build ./...` 通过
- `bes-selftest` 133/133 通过
- UA 版本与 TLS 握手预设版本一致
- `tls.peet.ws` 实测 JA3/JA4 与目标 Chrome 版本匹配
- `grep -r "CurlImpersonate\|CurlCffiClient\|buildChromeClientHelloSpec\|DefaultTLSProfiles" internal/` 无结果

### Phase 2：检测闭环 + 指纹引擎增强（P2）

#### 2.1 FingerprintJS 检测闭环

在 `cmd/bes-selftest/main.go` 新增一致性检查（非 API 存在性，而是值交叉校验）。

#### 2.2 指纹采样联合分布

让 GPU×OS×Screen 联合采样——从 apify 数据中按 OS 过滤后采样，而非独立。

### Phase 3：HTTP/3 + 文档修正（P3）

#### 3.1 HTTP/3

当前 `quic.go` 空壳。建议：保持 `CheckAvailable()=false`，从 roadmap 移除 HTTP/3 标记，不假装支持。真实需求出现时接入 quic-go。

#### 3.2 文档修正

- AGENTS.md：删除"三级降级链"，改为"纯 Go utls"
- AGENTS.md：更新依赖列表
- roadmap.md：HTTP/3 标记未完成，v8go 迁移标记已完成

---

## 4. 底层基座：切换/并存/保持决策

### 4.1 TLS 指纹：保持，不切换不并存

utls 是正确底座，问题全在使用方式。tls-client/CycleTLS/azuretls 底层全是 utls——与 bes 同根。bes 的 H2 帧级指纹比竞品更透明可控。切换到 tls-client 会失去 H2 帧级控制 + 引入 BSD-4-Clause + 一堆依赖。

**唯一值得"借鉴不引入"**：tls-client 的 Chrome profile 数据表（紧跟 Chrome 144），作为 utls 版本映射参考。

### 4.2 JS 引擎：保持

v8go（自行 fork+build）是正确选择。V8 的 ECMAScript 语义与真实 Chrome 一致是核心卖点。goja（纯 Go 无 CGO）可作为未来无 CGO 构建的双引擎后端，但指纹一致性会漂移。

### 4.3 反检测路线：保持核心，不并存真浏览器

bes 的 V8 Isolate 池化 + 零进程开销是核心卖点。引入真浏览器后端会模糊定位 + 引入进程管理复杂度。需要真实渲染的场景，用户应直接用 camoufox/patchright——bes 不覆盖所有场景。

**正确做法**：做好"上游预筛层"定位，明确标注能力边界。

---

## 5. 纯 V8 环境能力边界

### 5.1 第一层：原理性无法实现（没有渲染引擎，补不回来）

| 功能 | bes 实现 | 真实浏览器 | 检测后果 |
|------|---------|-----------|---------|
| Canvas 2D 渲染 | 合成 PNG（种子确定性生成） | GPU 光栅化，每台机器有像素差 | canvas hash 比对识别 |
| WebGL 渲染 | 预设 vendor/renderer 字符串 | GPU 渲染像素 hash | WebGL 渲染指纹无法伪造 |
| WebGPU 计算 | mock `navigator.gpu` | 真实 GPU 计算 | adapter 信息假，无真实 device |
| 字体渲染 | 字体名列表 only | 字形渲染像素差 | `measureText` 宽度无法匹配真实字体 |
| CSS 布局 | 全零尺寸，`getComputedStyle` 返回空 | 完整 Blink 布局引擎 | `offsetWidth > 0` 异常，媒体查询差分 |
| 图片解码 | 无 | JPEG/PNG/WebP/AVIF 解码 | 无 |

### 5.2 第二层：模拟了但与真实浏览器不一致

| 功能 | bes 实现 | 真实浏览器 | 能补吗 |
|------|---------|-----------|--------|
| WebSocket | ✅ 真实 TCP+TLS+WS 帧（RFC 6455 纯 Go，wsBridge 线程模型参照 Worker） | 真实 TCP+TLS+WS 帧 | ✅ 已完成 |
| WebRTC SDP | `createOffer()` 返回空 SDP | SDP 含真实 ICE candidate/IP | ⚡ 可生成假 SDP |
| Worker | ✅ 独立 V8 Isolate + inbound/outbound channel 双泵 | 独立 V8 Isolate + 线程 | ✅ 已完成 |
| Service Worker | 对象存在不运行 | 注册/运行 SW 脚本 | ⚡ 可模拟 |
| indexedDB | `open()` 永远触发 onerror | 真实 IndexedDB 存储 | ⚡ 可用 Go KV 实现 |
| 事件循环 | Go timers + FlushTimers 手动触发 | 原生 task/microtask queue 优先级 | ⚡ 可调优 |
| iframe 隔离 | `Object.create(window)` 半假 | 独立浏览上下文 | ⚡ 可改 |
| DOM 完整性 | `querySelector` 只处理 head/body/html | 完整 CSS 选择器引擎 | ⚡ 可逐步补 |
| ES Module | ✅ `importModule()` polyfill（data:/blob: URL，export default + named exports） | 原生支持 | ✅ 已完成 |
| HTTP/3 (QUIC) | 空壳 `CheckAvailable()=false` | 真实 QUIC 传输 | ⚡ 需 quic-go |
| TLS 版本切换 | 固定 Chrome133 | 按版本切换 ClientHello | ⚡ Phase 1 修复 |

### 5.3 实际验证结果（WSL 运行）

```
typeof WebAssembly      = object     ✅ V8 原生自带
WebAssembly.validate()  = true       ✅ WASM 编译执行可用
typeof eval             = function   ✅
typeof Function         = function   ✅
typeof Proxy            = function   ✅
typeof Reflect          = object     ✅
typeof Promise          = function   ✅
typeof WebAssembly.compile = function ✅
typeof crypto.subtle.digest = function ✅ 真实 SHA-256
typeof TextEncoder      = function   ✅
typeof btoa/atob        = function   ✅
typeof structuredClone  = function   ✅
typeof import()         = SyntaxError ❌ 非 module 模式
```

---

## 6. 逆向纯协议可行性分析

### 6.1 定义

典型流程：抓包 → 定位加密函数 → 抠出 JS → 沙箱执行 → 拿到签名/加密参数 → 配合 HTTP 请求。

核心需求：目标网站 JS 在 bes 沙箱里能跑通，不报错，产出正确加密参数。不需要真实渲染。

### 6.2 逆向场景 JS 环境依赖分层

| 依赖层 | 出现频率 | bes 状态 | 覆盖 |
|--------|---------|---------|------|
| 纯 JS 运算（AES/RSA/SM2/SM4/MD5/SHA） | ~95% | ✅ V8 原生 | ✅ |
| WebAssembly（WASM 签名/加密） | ~15%，趋势上升 | ✅ V8 原生自带 | ✅ |
| eval/Function/Proxy/Reflect（动态执行/混淆） | ~80% | ✅ V8 原生 | ✅ |
| crypto.subtle（Web Crypto API） | ~10% | ✅ Go 回调真 SHA-256 | ✅ |
| crypto.getRandomValues | ~30% | ✅ Go crypto/rand 真随机 | ✅ |
| atob/btoa/TextEncoder/TextDecoder | ~60% | ✅ JS 实现 | ✅ |
| navigator/screen/window 属性 | ~90% | ✅ 指纹引擎灌入 | ✅ |
| setTimeout/setInterval | ~50% | ✅ Go timers + FlushTimers | ✅ |
| XMLHttpRequest/fetch | ~40% | ✅ 接 netlayer 真发请求 | ✅ |
| document.cookie | ~30% | ✅ CookieStore 完整模拟 | ✅ |
| localStorage/sessionStorage | ~20% | ✅ 完整 Storage API | ✅ |
| document.URL/location.href | ~70% | ✅ 可配置 | ✅ |
| window.chrome 对象 | ~25% | ✅ 注入 | ✅ |
| toString 原生格式 | ~20% | ✅ nativeFns spoofing | ✅ |
| Worker（加密在 Worker 里） | ~5% | ❌ 空壳不执行 | ❌ |
| ES Module import() | ~8% | ❌ SyntaxError | ❌ |
| Canvas 指纹作为签名输入 | ~3% | ⚠️ 合成 PNG | ⚠️ |
| WebSocket 协议 | ~2% | ❌ 空壳不连 | ❌ |

### 6.3 覆盖率估计

```
能直接跑通：          ~85-90%
需补环境迭代后跑通：  ~5-10%
原理性跑不通：        ~3-5%
```

### 6.4 bes 路线优势（vs Node vm 补环境）

1. **零 Node 痕迹**：V8 Isolate 天然无 Buffer/process/require
2. **WebAssembly 原生可用**：V8 自带，无需额外工作
3. **指纹自洽**：navigator/UA/screen/cookie/location 一致性内置
4. **XHR/fetch 接真网络**：加密后直接发请求，闭环在沙箱内
5. **CDP 调试**：chrome://inspect 直连

### 6.5 逼近"所有"需补的 3 件事

| 优先级 | 补什么 | 工程量 | 覆盖率提升 |
|--------|--------|--------|-----------|
| P0 | Worker 真实执行（v8go 子 Isolate + postMessage） | 中 | +5% → ~93% |
| P0 | ES Module 支持（import()/import xxx from） | 中 | +8% → ~96% |
| P1 | WebSocket 真实连接（Go websocket + JS 桥接） | 小 | +2% → ~98% |

剩余 ~2% 是 Canvas/WebGL 渲染指纹作为签名输入——**不是原理天花板，有 4 个不引入浏览器的方案**（见第 7 章）。

---

## 7. 渲染指纹补充方案（不引入浏览器）

> 核心矛盾：渲染指纹需要"真实像素输出"，但引入浏览器进程就否定了 bes 的存在意义。
> 以下 4 个方案均**不启动浏览器进程**，按成本从低到高排列，可分层组合。

### 7.1 问题本质

目标 JS 执行了类似代码：

```javascript
var canvas = document.createElement('canvas');
ctx.fillText('某固定字符串');
var hash = md5(canvas.toDataURL());  // ← 渲染结果作为签名输入
sign = hash + timestamp + ...
```

检测端要的不是"会调 `toDataURL()`"，而是 **`toDataURL()` 返回的像素 hash 是否与真实浏览器一致**。

bes 现状：返回种子合成的假 PNG（`knowledge_base.go:394`），hash 永远对不上。

### 7.2 方案 1：预采集 + 回放（零依赖，覆盖 ~80% Canvas 签名场景）

**原理**：绝大多数网站的 canvas 签名绘制的是**固定内容**（固定字符串/固定图形），不随用户变化。渲染结果只取决于 Chrome 版本 × OS × GPU 组合。

**做法**：
1. 用真实 Chrome 在几组常见组合下（Chrome152+Win11+RTX4060 等），采集 `toDataURL()` 输出
2. 存成数据集，按 `chrome版本_OS_GPU` 索引
3. bes 沙箱中 `toDataURL()` 按当前指纹组合返回预采集值

| | 评价 |
|---|---|
| 优点 | 零 CGO、零新依赖、保持纯 Go。返回的是**真实渲染结果**，hash 与真实浏览器一致 |
| 局限 | 只覆盖"绘制固定内容"的场景。动态内容（如 `ctx.fillText(username + timestamp)`）对不上 |
| 工程量 | 小——采集脚本 + 数据集 + `toDataURL()` 拦截替换 |
| 引入浏览器？ | ❌ 不引入（采集是一次性离线动作，运行时纯查表） |

### 7.3 方案 2：Canvas 2D 软件渲染（覆盖动态内容场景）

**原理**：Chrome 自己的 Canvas 2D 底层是 **Skia**，Skia 有纯 CPU 软件后端（不需要 GPU）。Chrome 在无 GPU 环境下就用软件渲染。

**做法**：
- CGO 绑定 Skia 的软件光栅化路径（或更轻的 Cairo）
- FreeType 做字体渲染（Chrome 自己也用 FreeType）
- `ctx.fillText('动态内容')` → 真实光栅化 → `toDataURL()` 返回真实渲染的 PNG

| | 评价 |
|---|---|
| 优点 | 覆盖动态绘制内容。返回的是**真实光栅化结果**，字体度量、抗锯齿、像素分布都真实 |
| 局限 | 渲染 hash 与 GPU Chrome **不完全一致**（CPU vs GPU 光栅化像素微差）。检测端只验"是否有效渲染"能过；预存 hash 严格比对过不了（方案 1 反而能过） |
| 工程量 | 大——Skia CGO 绑定 + Canvas API 桥接 + 字体管理 |
| 引入浏览器？ | ❌ 不引入（Skia/Cairo/FreeType 是进程内 C/C++ 库，不是浏览器进程） |
| 新增依赖 | Skia 或 Cairo（CGO）、FreeType（CGO），体积较大 |

### 7.4 方案 3：WebGL 软件渲染（SwiftShader）

**原理**：Google 自己开发了 **SwiftShader**——纯软件的 Vulkan/GL 实现。Chrome 在无 GPU 时就用它。不需要 GPU 硬件。

**做法**：
- SwiftShader 提供 EGL 离屏上下文（不需要窗口）
- CGO 绑定，创建离屏 GL context
- `gl.drawArrays()` → 软件光栅化 → `gl.readPixels()` 读真实像素

| | 评价 |
|---|---|
| 优点 | WebGL 渲染指纹是**真实的软件渲染结果**，不是预设字符串 |
| 局限 | SwiftShader 渲染结果与 GPU 渲染**不同**——但真实 Chrome 在无 GPU 机器上也用 SwiftShader，是一种"合法"渲染路径 |
| 工程量 | 大——SwiftShader CGO 绑定 + WebGL API 桥接 |
| 引入浏览器？ | ❌ 不引入（SwiftShader 是进程内 C++ 库，不是浏览器进程） |
| 新增依赖 | SwiftShader（~20MB，编译复杂） |

### 7.5 方案 4：Canvas API 录制 + 远程渲染（本地不引入浏览器）

**原理**：沙箱中拦截 Canvas 2D API 调用，把绘制指令序列化，发送到远程渲染服务（运行真实 Chrome+GPU），返回真实渲染结果。

**做法**：
- 拦截 `ctx.fillText()`/`ctx.arc()`/`ctx.drawImage()` 等调用，记录指令序列
- `toDataURL()` 触发时，把指令序列发到远程渲染 API
- 远程用真实 Chrome + 指定 GPU 渲染，返回 PNG

| | 评价 |
|---|---|
| 优点 | 渲染 hash 与真实 GPU Chrome **完全一致**。不在本地引入浏览器进程 |
| 局限 | 引入网络依赖 + 远程服务运维。但远程服务不影响 bes 本身的轻量定位 |
| 工程量 | 中——指令序列化 + 远程渲染 API + 缓存 |
| 引入浏览器？ | ❌ 本地不引入（远程服务是独立组件，不影响 bes 架构） |

### 7.6 组合策略

不需要选一个——分层覆盖：

```
方案 1（预采集回放）    ← 默认，零依赖，覆盖 80% Canvas 固定内容场景
    ↓ 未命中（动态内容）
方案 2（Skia 软件渲染） ← 覆盖动态 Canvas 场景
    ↓ 需要严格 hash 匹配
方案 4（远程渲染）      ← 最后兜底，hash 完全一致
```

WebGL 场景同理：方案 1 预采集 + 方案 3 SwiftShader + 方案 4 远程渲染。

**bes 本身仍然是纯 V8 + Go，不引入浏览器进程。渲染能力按需挂载为可选模块，不是核心路径。**

### 7.7 场景覆盖与方案映射

| 场景 | 占比 | 推荐方案 | 引入浏览器？ | 新增依赖 |
|------|------|---------|------------|---------|
| Canvas 固定内容签名 | ~1.5% | 方案 1 预采集回放 | ❌ | 零 |
| Canvas 动态内容签名 | ~0.3% | 方案 2 Skia 软件渲染 | ❌ | Skia/FreeType CGO |
| WebGL 渲染签名 | ~0.15% | 方案 3 SwiftShader / 方案 1 预采集 | ❌ | SwiftShader CGO |
| 严格 hash 匹配 | ~0.05% | 方案 4 远程渲染 | ❌ 本地不引入 | 网络服务 |

### 7.8 结论

这 2% 可以不引入浏览器。方案 1（预采集）零依赖就能覆盖大部分，剩余少数动态场景用软件渲染库（Skia/SwiftShader）补——这些是进程内 C/C++ 库，不是浏览器。只有极端严格 hash 匹配才需要远程渲染，但那是网络服务不是本地浏览器。

**bes 的意义不会消失**——它仍然是"纯 V8 + Go 的轻量执行引擎"，渲染只是按需挂载的可选模块，不是核心路径。

### 7.9 方案 1 比 bes 现有合成 PNG 更轻量

| | bes 现状（合成 PNG） | 方案 1（预采集回放） | 真浏览器 |
|---|---|---|---|
| 运行时开销 | 每次 `toDataURL()` 调用都要 SHA256 + 构造 PNG 字节 | **查表返回预存字符串，零计算** | 启动 GL context + GPU 光栅化 |
| 内存 | ~1KB/次（临时生成） | 数据集常驻 ~几 MB | ~200MB/实例 |
| CPU | 有（hash + 字节构造） | **几乎为零** | 有（渲染管线） |
| 依赖 | 无 | 无 | 无 |
| 返回值真假 | 假（合成字节） | **真（真实 Chrome 采集的输出）** | 真 |

方案 1 本质是把 `toDataURL()` 从"运行时生成"改成"查表返回"——代码反而更简单，开销反而更低，结果反而更真。

方案 2/3（Skia/SwiftShader）有内存开销（Skia ~5MB，SwiftShader ~20MB），但和浏览器比：

```
bes + 方案1（预采集）         →  内存增量 ≈ 0        覆盖 ~80% Canvas 场景
bes + 方案2（Skia）           →  +5MB CGO 库        覆盖动态 Canvas 场景
bes + 方案3（SwiftShader）    →  +20MB CGO 库       覆盖 WebGL 场景
真浏览器（Chrome headless）    →  +200MB/实例        覆盖所有场景
```

即使方案 2+3 都挂上（~25MB），仍是一个 Chrome 实例（200MB）的八分之一。且 Skia/SwiftShader **按需加载**——不调 Canvas/WebGL 就不初始化，不影响核心 V8 路径。

---

## 8. 架构模式：通用引擎 + 外部补环境分离

### 8.1 bes 现有架构已是分离设计

bes 的 API 层已内置完整的"外部脚本注入"机制：

```go
// internal/bridge/server.go:96-97
Preload []string `json:"preload"` // 脚本文件路径，创建 session 时自动加载
Init   string   `json:"init"`    // JS 代码，preload 后执行
```

调用方式：

```bash
# 创建 session 时预加载网站专用补环境脚本
POST /api/session
{
  "browser": "chrome", "os": "windows",
  "preload": ["/path/to/site-x-env.js"],
  "init": "window.__siteConfig = { apiVersion: 'v2' }"
}

# 运行时动态加载
POST /api/session/{id}/script
{ "name": "site-x-sign.js", "content": "..." }
```

bes 自身的 `env_shim_part1-5.js` 是**通用补环境**（navigator/DOM/Crypto/Storage 等所有网站都需要的），网站特定逻辑通过 `preload`/`script` 端点从外部注入——架构已经是分离的。

### 8.2 渲染指纹补全走同一架构

渲染补全作为**独立项目仓库**提供，通过 bes 的 preload 机制注入：

```
bes-repo/                    ← 通用引擎（纯 V8 + Go）
  internal/sandbox/          ← 通用补环境 env_shim_part1-5.js
  API: preload/init/script   ← 注入入口（已实现）

bes-site-profiles/           ← 独立仓库（网站特定补环境）
  sites/
    site-x/
      env.js                 ← 站点 X 专用补环境
      canvas-dataset.json    ← 站点 X 预采集的 Canvas 渲染结果
      sign.js                ← 站点 X 签名调用入口
```

`canvas-dataset.json` 存方案 1 的预采集数据，`env.js` 做拦截替换：

```javascript
// site-x/env.js — 通过 preload 注入 bes 沙箱
(function() {
  var dataset = { /* 预采集的 toDataURL 结果 */ };
  var origToDataURL = HTMLCanvasElement.prototype.toDataURL;
  HTMLCanvasElement.prototype.toDataURL = function() {
    var key = detectDrawPattern(this);  // 识别当前 canvas 画了什么
    if (dataset[key]) return dataset[key];
    return origToDataURL.call(this);    // 未命中降级到 bes 默认
  };
})();
```

### 8.3 方案 2（Skia）也可走同一架构

Skia CGO 绑定做成 bes 的可选 plugin（build tag 控制），网站专用仓库通过 preload 调用：

```javascript
// 如果 bes 编译了 Skia plugin，走真实软件渲染
// 否则降级到预采集数据
if (typeof bes.canvasRender === 'function') {
  // bes.canvasRender(text) → Go 回调 → Skia 光栅化 → 返回 PNG
} else {
  return dataset[key]; // 预采集回放
}
```

### 8.4 分离架构的好处

| 好处 | 说明 |
|------|------|
| bes 核心零改动 | 渲染补全逻辑全在独立仓库的 JS 里，通过 preload 注入 |
| 网站特定逻辑不污染通用引擎 | 每个网站的 Canvas 数据集、补环境逻辑独立维护 |
| bes 定位保持纯粹 | 通用 V8 沙箱 + 通用浏览器环境，不做网站特定适配 |
| 可选依赖 | Skia/SwiftShader 通过 build tag 控制，不编译就不增加体积 |
| 独立仓库可独立迭代 | 网站补环境脚本更新不需要重新编译 bes |

---

## 9. 行动项总表

| 优先级 | 行动项 | 维度 | 状态 |
|--------|--------|------|------|
| ✅ | v8go 迁移到 tommie fork + 自行 build | 沙箱 | 已完成 |
| ✅ | TLS 降级链修正为纯 utls | TLS | 已完成 |
| ✅ | 清理 curl_impersonate.go / tls_spec.go / tls_profile.go / quic.go 死代码 | TLS | 已完成（2026-08-29） |
| ✅ | utls 预设按 Chrome 版本动态切换 + PQ 补丁条件注入 | TLS | 已完成（2026-08-29） |
| ✅ | H2/UA 版本对齐（删硬编码默认 UA）+ tlsTarget per-session 透传 | TLS | 已完成（2026-08-29） |
| **P0** | Worker 真实执行（v8go 子 Isolate） | 沙箱 | 待执行 |
| **P0** | ES Module 支持（import()） | 沙箱 | 待执行 |
| **P1** | WebSocket 真实连接（Go websocket + JS 桥接） | 沙箱 | 待执行 |
| **P1** | Canvas 预采集回放（方案 1：数据集 + toDataURL 拦截替换） | 渲染 | 待执行 |
| ✅ | AGENTS.md / roadmap.md 文档修正 | 全局 | 已完成（2026-08-29） |
| ✅ | internal/session 孤儿包处置（Manager 删除 + ProfileStore 接线 bridge） | 架构 | 已完成（2026-08-29） |
| **P2** | FingerprintJS + CreepJS 嵌入 selftest 检测闭环 | 指纹 | 待执行 |
| **P2** | 指纹采样联合分布（GPU×OS×Screen） | 指纹 | 待执行 |
| **P2** | 采 tls-client 开源 profile 数据补 Chrome 版本映射 | TLS | 待执行 |
| **P3** | Canvas 2D 软件渲染（方案 2：Skia/FreeType CGO 绑定） | 渲染 | 按需 |
| **P3** | WebGL 软件渲染（方案 3：SwiftShader CGO 绑定） | 渲染 | 按需 |
| **P3** | HTTP/3 QUIC 实现（需 quic-go 依赖） | TLS | 按需 |
| **P3** | 借鉴 apify datapoints Bayesian 频率采样 | 指纹 | 按需 |
| **P4** | Canvas 远程渲染服务（方案 4：指令序列化 + 远程 API） | 渲染 | 极端场景 |

---

## 附：数据置信度说明

- **高置信**：各项目 GitHub stars/更新时间/归档状态（`gh api` 精确抓取）、bes 代码实读、WSL 运行验证
- **中置信**：覆盖率估计（基于逆向经验，非统计数据）、各库 License 版本
- **未核实**：各库实际过检率（需实时测试）、bes 自行 fork 的 v8go 与 tommie fork 的 API 兼容细节
