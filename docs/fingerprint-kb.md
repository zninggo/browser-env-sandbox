# 指纹知识库 (Fingerprint Knowledge Base)

> 指纹引擎的核心数据源。定义浏览器 × OS × 硬件的属性矩阵，确保生成的指纹自洽。

## 1. 浏览器维度

### Chrome (主要目标)

| 版本范围 | UA-CH brands | 关键差异 |
|---------|-------------|---------|
| 120-123 | Chromium 12X, Google Chrome 12X, Not.A/Brand 24 | userAgentData 引入 |
| 124-128 | 同上 | — |
| 129-131 | 同上 | — |
| 132-135 | 同上 | — |

每个版本需要记录：
- `navigator.userAgent` 模板
- `navigator.userAgentData.brands` 数组
- `navigator.appVersion`
- V8 引擎版本号 (参与某些指纹)
- Blink 版本号

### Firefox (次要)

- `navigator.userAgent` 模板
- 无 `window.chrome` 对象
- `navigator.userAgentData` = undefined
- 不同的 canvas 渲染路径

### Safari (次要)

- 无 `window.chrome`
- `navigator.platform` = MacIntel
- 独特的 WebKit 行为

## 2. OS 维度

### Windows

| 属性 | Win10 | Win11 |
|------|-------|-------|
| navigator.platform | Win32 | Win32 |
| navigator.userAgent OS 段 | Windows NT 10.0 | Windows NT 10.0 (相同) |
| 默认字体 | Arial, Calibri, ... | 同 + Segoe UI Variable |
| screen.colorDepth | 24 | 24 |
| 文件路径分隔符 | \ | \ |

Win10 vs Win11 在 UA 中无法区分 (都报 NT 10.0)，但字体集和 GPU 驱动有差异。

### macOS

| 属性 | 值 |
|------|-----|
| navigator.platform | MacIntel |
| UA OS 段 | Macintosh; Intel Mac OS X 10_15_7 |
| 默认字体 | Helvetica, San Francisco, ...
| screen.colorDepth | 30 (部分) 或 24 |

### Linux

| 属性 | 值 |
|------|-----|
| navigator.platform | Linux x86_64 |
| UA OS 段 | X11; Linux x86_64 |
| 默认字体 | DejaVu, Liberation, ... |

### Android (移动端)

| 属性 | 值 |
|------|-----|
| navigator.platform | Linux armv8l |
| UA | ...Android 13; ... |
| maxTouchPoints | 5 |
| 默认字体 | Roboto, Noto, ... |

## 3. GPU 维度

### NVIDIA

| GPU | WebGL UNMASKED_RENDERER | canvas 影响 |
|-----|------------------------|-------------|
| RTX 3060 | ANGLE (NVIDIA, NVIDIA GeForce RTX 3060 ...) | hash A |
| RTX 4060 | ANGLE (NVIDIA, NVIDIA GeForce RTX 4060 ...) | hash B |
| RTX 4090 | ANGLE (NVIDIA, NVIDIA GeForce RTX 4090 ...) | hash C |

### Intel

| GPU | WebGL UNMASKED_RENDERER |
|-----|------------------------|
| UHD 630 | ANGLE (Intel, Intel(R) UHD Graphics 630 ...) |
| Iris Xe | ANGLE (Intel, Intel(R) Iris(R) Xe Graphics ...) |

### AMD

| GPU | WebGL UNMASKED_RENDERER |
|-----|------------------------|
| RX 6700 XT | ANGLE (AMD, AMD Radeon RX 6700 XT ...) |

### Apple (macOS)

| GPU | WebGL UNMASKED_RENDERER |
|-----|------------------------|
| M1 | ANGLE (Apple, ANGLE Metal Renderer: Apple M1) |
| M2 | ANGLE (Apple, ANGLE Metal Renderer: Apple M2) |

## 4. 自洽约束矩阵

选了 OS + GPU + 浏览器后，以下属性自动确定：

```
OS=Windows + GPU=NVIDIA RTX4060 + Chrome 131
  ├─ navigator.platform = Win32
  ├─ navigator.userAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) ... Chrome/131 ..."
  ├─ WebGL renderer = "ANGLE (NVIDIA, NVIDIA GeForce RTX 4060 ...)"
  ├─ 字体集 = Windows 默认 + NVIDIA 驱动字体
  ├─ canvas hash = f(Chrome131, RTX4060, Win字体)  ← 需要预采集 hash 库
  ├─ AudioContext hash = f(Chrome131, Windows)     ← 需要预采集
  ├─ screen.colorDepth = 24
  └─ navigator.maxTouchPoints = 0
```

## 5. 需要预采集的 hash 库

以下指纹的 hash 值无法纯计算得出，需要从真机采集：

- **canvas toDataURL hash** — 各 GPU × Chrome 版本 × OS 组合
- **canvas measureText hash** — 各 OS × Chrome 版本
- **AudioContext fingerprint hash** — 各 OS × Chrome 版本
- **WebGL 参数组合** — 各 GPU 的完整 getParameter 返回值集

采集方式：在真机上跑采集脚本 → 存入知识库 → 生成时查表。

## 6. 时区 + 语言 + IP 地理联动

```
IP 地理 = 中国上海
  ├─ 时区 = Asia/Shanghai (UTC+8)
  ├─ Intl.DateTimeFormat.resolvedOptions().timeZone = "Asia/Shanghai"
  ├─ navigator.languages = ["zh-CN", "zh"]
  ├─ Date.toString() 时区偏移 = +0800
  └─ WebRTC IP = 代理 IP (需匹配地理)
```

IP 地理与时区/语言不一致是最常见的检测点之一。
