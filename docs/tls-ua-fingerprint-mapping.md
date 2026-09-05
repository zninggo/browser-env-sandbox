# TLS ↔ UA 指纹强绑定映射表

> 本文档是 `internal/netlayer` 的 TLS / JA3 / JA4 / HTTP2 指纹映射表，与
> `internal/netlayer/chrome_fingerprint.go` 中的 `chromeFingerprintLadder` 一一对应。
> 目标：给定一个 User-Agent，能查到它声明的 Chrome 版本对应的 ClientHello 预设与
> HTTP/2 帧指纹，且二者与该 UA 一致 —— **不再随机生成**。

## 1. 绑定链路

```
User-Agent 字符串
  └─ 解析 Chrome 主版本号        chromeVersionFromUA(ua)
       └─ 查表得到指纹 profile     chromeProfileFor(major)
            ├─ utls.ClientHelloID  → 决定 JA3 / JA4（ClientHello 字节）
            ├─ pqSigAlgs           → Chrome 140+ 追加 ML-DSA/SLH-DSA 签名算法
            └─ h2Fingerprint       → 决定 HTTP/2 SETTINGS/PRIORITY（akamai 指纹）
```

UA 中的 Chrome 版本号与 TLS/HTTP2 指纹取自同一份 `Browser.Version`（`fpengine` 生成
UA 模板时写入），netlayer 侧再以 UA 版本号为准查表，保证「UA 说 Chrome 150 →
ClientHello 就是 Chrome 150 的样子」。

## 2. Chrome 版本 → ClientHello 预设（JA3/JA4 来源）

每条预设的精确字段（密码套件顺序、扩展顺序、加密/椭圆曲线、key share、签名算法、
ALPN）定义在 `refraction-networking/utls v1.8.2` 的 `u_parrots.go`，对应
`case HelloChrome_NNN` 代码块。JA3/JA4 由这些字段**确定性**推导而来（见 §4），
因此选对预设即选对 JA3/JA4，没有随机项。

| Chrome 主版本 | utls 预设 (ClientHelloID) | 该版本的 ClientHello 关键差异 |
|---|---|---|
| 58–61 | `HelloChrome_58` | TLS ≤1.2，TLS 1.3 之前的密码套件顺序 |
| 62–69 | `HelloChrome_62` | 与 58 同 spec（utls 内同一定义） |
| 70–71 | `HelloChrome_70` | 首个默认 TLS 1.3 ClientHello |
| 72–82 | `HelloChrome_72` | 密码套件/扩展重排 |
| 83–86 | `HelloChrome_83` | ALPS + 新证书压缩 codepoint |
| 87–95 | `HelloChrome_87` | GREASE 在密码套件列表中的位置变化 |
| 96–99 | `HelloChrome_96` | 密码套件重排 |
| 100–101 | `HelloChrome_100` | SupportedVersions 移除 TLS 1.0/1.1 |
| 102–105 | `HelloChrome_102` | 签名算法/扩展 shuffle |
| 106–114 | `HelloChrome_106_Shuffle` | GREASE 扩展洗牌（106 起） |
| 115–119 | `HelloChrome_115_PQ` | 引入 X25519Kyber768 后量子 key share |
| 120–130 | `HelloChrome_120_PQ` | Kyber768 + GREASE EncryptedClientHello |
| 131–132 | `HelloChrome_131` | Kyber→ML-KEM-768 切换 |
| 133+ | `HelloChrome_133` | 新 ALPS codepoint |

**回退规则（rollback）**：Chrome 只在上述特定版本改 ClientHello，两个相邻预设之间
的版本复用「最近的 ≤ 自身版本」的预设。这与 JA3/JA4 指纹库（ja3er / tlsfingerprint.io）
的版本标注约定一致 —— 同一 JA3/JA4 hash 覆盖一段连续的 Chrome 版本。

**后量子签名算法（独立轴）**：Chrome 140+ 在 `signature_algorithms` 前插
ML-DSA-65(0x0904)/ML-DSA-87(0x0905)/SLH-DSA(0x0906)。utls v1.8.2 的 Chrome 预设止于
133，未包含这些，故在 `dialUTLS` 中按 `profile.pqSigAlgs`（`chromeFingerprintFor` 对
≥140 置位）单独注入，见 `internal/netlayer/tls_spec.go`。

## 3. HTTP/2 帧指纹（akamai）

Chrome 的 HTTP/2 前导（preface）自 v57 起基本未变，实抓 Chromium 151
（tls.peet.ws）得到：

```
akamai_fingerprint = 1:65536;2:0;4:6291456;6:262144|15663105|0|m,a,s,p
```

| 帧 | 字段 | 值 | 说明 |
|---|---|---|---|
| SETTINGS | HEADER_TABLE_SIZE (0x1) | 65536 | |
| SETTINGS | ENABLE_PUSH (0x2) | 0 | |
| SETTINGS | INITIAL_WINDOW_SIZE (0x4) | 6291456 | 6 MiB |
| SETTINGS | MAX_HEADER_LIST_SIZE (0x6) | 262144 | |
| WINDOW_UPDATE | connection (stream 0) | 15663105 | |
| HEADERS | PRIORITY (exclusive, weight) | weight=256（帧内存 255） | `m,a,s,p` 指权重/流依赖 |

对应代码 `internal/netlayer/h2client.go` 中 `h2Fingerprint`（默认值
`chromeH2Default`）。因该值长期稳定，本梯队所有条目指向同一默认值；若未来 Chrome
改动，只需在表内改一行。

## 4. JA3 / JA4 如何由预设确定性导出

- **JA3** = MD5( 密码套件列表 ‖ 扩展类型码列表 ‖ 椭圆曲线列表 ‖ EC 点格式列表 )。
  全部来自所选 `ClientHelloID` 的 `ClientHelloSpec`。
- **JA4** = 基于（协议版本、SNI、密码套件数量+列表、扩展数量+唯一化列表、ALPN、
  签名算法集 hash）的紧凑编码。签名算法集即决定 JA4 中 `sig_algs` 分量；Chrome 140+
  注入 PQ 算法正是为了对齐该分量。

因此「版本 → 预设」的映射天然锁死 JA3 与 JA4；测试
`internal/netlayer/chrome_fingerprint_test.go` 验证了回退规则、PQ 阈值、确定性（同一
版本/UA 重复解析结果恒等）与表单调性。

## 5. 来源与可复现性

- **ClientHello 字段**：`refraction-networking/utls v1.8.2` `u_parrots.go`（逐版本
  `case HelloChrome_NNN` 的 `ClientHelloSpec`）。
- **版本↔预设对应**：Chrome 官方发布历史（各版本 ClientHello 首次出现的版本）。
- **HTTP/2 帧 + PQ 签名算法**：Chromium 151 实抓（tls.peet.ws akamai_fingerprint）。
- **js/页面侧 UA 一致性**：`fpengine` 的 `BrowserProfile.Version` 同时写入 UA 模板与
  `userAgentData.brands`，netlayer 再据此版本查表。

已知限制：utls v1.8.2 的 Chrome 预设止于 133；133 之后的 Chrome（134–150）其
ClientHello 除 PQ 签名算法外无其他可观察差异，故统一映射到 `HelloChrome_133` 并单独
处理 PQ 算法。若 utls 后续发布更新预设，需同步扩展本表。