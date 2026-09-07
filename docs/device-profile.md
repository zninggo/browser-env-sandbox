# 指纹数据驱动（device profile 联合分布）

> 本文档说明 fpengine 如何从「规则生成」升级为「数据驱动」：把 `seed → 规则模板`
> 换成「从真实设备的联合分布采样」。配套代码：
>
> - `internal/fpengine/device_profile.go` —— device profile 数据模型 + 联合采样器
> - `internal/fpengine/device_profile_loader.go` —— 运行时数据加载
> - `internal/fpengine/device_profile_engine.go` —— 引擎接入（`Engine.Generate` 首选路径）
> - `internal/fpupdate/parser_deviceprofile.go` —— 把 apify 贝叶斯网络转成联合分布（建模管线）
> - `data/device_profiles.json` —— 生成的联合分布数据文件（已提交）

## 一、问题

旧实现里，`seed` 只是驱动一个「规则模板」：浏览器 / OS / GPU / 分辨率 / CPU / 内存 /
字体各自**独立**采样，只做 OS 级别的粗过滤。真实设备不是这样——一台 Apple M 系笔记本
**同时**具备 Metal 渲染器、Retina 屏幕、Apple 字体、8~10 核、DPR=2；独立采样会产出
「Apple M2 GPU + Windows D3D11 屏幕 + 24 核桌面 CPU」这类物理上不可能的组合，任何
交叉验证的风控一验就穿帮。

## 二、样本来源

联合分布来自一个公开的开源指纹样本集，而非手工设定：

- **数据源**：`apify/fingerprint-suite` 的 `network.json`（Apache-2.0 许可）。
  - npm 包：`fingerprint-generator`，本次使用版本 **2.1.88**（打包于
    `data_files/fingerprint-network-definition.zip`）。
- **模型性质**：`network.json` 是一个**贝叶斯网络**，25 个节点，根节点是
  `userAgent`（235 个真实 UA），其余维度（videoCard/screen/fonts/
  hardwareConcurrency/deviceMemory/platform/vendor/maxTouchPoints …）都以
  `userAgent` 为条件父节点，带**条件概率表**。

也就是说，「联合分布」不是我们建模出来的，而是这份开源数据里**本来就有的**：
UA 决定 OS 与设备类型，进而决定该 OS 下各维度的观测频率。旧解析器（`parser.go`）
把它拍平成互相独立的全局列表，丢掉了相关性；本次新增的解析器保留条件结构。

自采（本地隔离真实浏览器）：另用本机 headless Chrome 采集了一个真实锚点设备，见
[§六 自采样本](#六-自采样本)。

## 三、建模方法

条件变量选择：**User-Agent**。UA 同时定位浏览器版本、OS、设备类型（桌面/移动），是
让 canvas hash / WebGL / 字体 / 分辨率 / CPU / 时区-语言 呈现实设备共现关系的最小
充分条件变量。

建模流程（`parser_deviceprofile.go`）：

1. 解析 `network.json` 的 25 节点贝叶斯网络。
2. 取根节点 `userAgent` 的 `possibleValues`（UA 字符串）与先验概率
   `conditionalProbabilities[ua]`。
3. 对每个公共子维度（`videoCard` / `screen` / `fonts` /
   `hardwareConcurrency` / `deviceMemory` / `platform` / `vendor` /
   `maxTouchPoints`），读取其 `conditionalProbabilities["deeper"][ua]` 的
   `{取值: 概率}` 条件分布。
4. 字体列表跨 UA 去重进一个共享 pool，条件表用池索引引用（929 个去重字体集）。
5. 输出 `data/device_profiles.json`（`user_agents` + `ua_priors` + 各维度的
   UA 条件加权类别分布）。

采样（`device_profile.go`）：给定 `seed → 随机数流`，先按（browser/os 约束筛选后的）
UA 先验抽一个 UA，再从该 UA 条件的每个维度分布里各抽一个值。**一次 UA 抽样，全体
维度一起确定**——这就是联合采样取代独立采样的核心。

## 四、数据模型

```go
// DeviceProfile = 一次联合采样得到的自洽设备画像
type DeviceProfile struct {
    UserAgent           string       // 条件变量（浏览器版本 + OS + 设备类型）
    Browser             string       // "chrome" | "firefox" | "safari"
    BrowserVersion      string       // 主版本号，如 "147"
    BrowserMajor        int
    OS                  string       // "windows" | "macos" | "linux" | "android"
    Platform            string       // navigator.platform，如 "Win32"
    Vendor              string       // navigator.vendor
    GPUDevice                      // WebGL renderer + vendor
    Screen             DeviceScreen // 分辨率 + colorDepth + DPR + outer 尺寸
    Fonts              []string
    HardwareConcurrency int
    DeviceMemory       int
    MaxTouchPoints     int
}
```

磁盘格式见 `deviceProfileData`（`user_agents` / `ua_priors` / `video_card` /
`screen` / `fonts{pool,cond}` / `hardware_concurrency` / `device_memory` /
`platform` / `vendor` / `max_touch_points`）。

## 五、相关性证据（真实观测频率，非设定）

对一份真实 Windows Chrome UA 与 macOS Chrome UA 的条件分布抽样（`network.json`
原始值），观测到的共现关系：

| 维度 | Windows (Win64 Chrome) | macOS (Chrome) | Android (Chrome) |
|------|------------------------|----------------|------------------|
| WebGL renderer | `ANGLE (..., Direct3D11 ...)` (Intel/NVIDIA/AMD) | `ANGLE (Apple, ... Metal Renderer: Apple M1/M2/M3/M4)` | `Mali-*` / `Adreno (TM) *` / `PowerVR` |
| 分辨率 | 1536×864 / 1920×1080 @ DPR 1/1.25/1.5 | 1512×982 / 1470×956 Retina @ DPR 2 | 360×800 / 393×873 @ DPR 2.75~3 |
| colorDepth | 32 / 24 | 30 | 24 |
| hardwareConcurrency | 8(25.9%)/12/4/16 | 8(33.9%)/10(27.8%) | 8(72.3%) |
| deviceMemory | 16(42%)/8/32 | 16(52%)/8/32 | 8(56%)/4 |
| platform | `Win32` (100%) | `MacIntel` (99.8%) | `Linux armv8l/aarch64` |
| maxTouchPoints | 0 (87.8%) | 0 (100%) | 5 (96.8%) |

整套维度在一次采样中共现，正是「canvas/WebGL/字体/分辨率/CPU 呈现真实相关性」的
样本支撑。集成测试 `device_profile_integration_test.go` 对每个 OS 采样数百次断言：

- macOS → 永不含 `Direct3D11`，renderer 为 Metal / OpenGL / Apple；
- Windows → 永不含 `Metal`，renderer 为 `Direct3D11`；
- Android → 永不含 `Direct3D11` / `Metal`，renderer 为 Mali/Adreno/PowerVR；
- 采样到的每个 renderer / 分辨率 / colorDepth 都落在源分布集合内（in-distribution）。

## 六、自采样本

按「自采仅用本地隔离的真实浏览器，不触碰线上生产目标」约束，用本机 headless Chrome
（`--headless=new --no-sandbox --disable-gpu`）采集了一个真实锚点设备：

```json
{
  "userAgent": "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) HeadlessChrome/152.0.0.0 Safari/537.36",
  "platform": "Linux x86_64",
  "hardwareConcurrency": 4,
  "deviceMemory": 16,
  "maxTouchPoints": 0,
  "screen": { "width": 800, "height": 600, "colorDepth": 24, "devicePixelRatio": 1 },
  "webgl": { "vendor": "Google Inc. (Google)",
             "renderer": "ANGLE (Google, Vulkan 1.3.0 (SwiftShader Device (LLVM 10.0.0) (0x0000C0DE)), SwiftShader driver)" },
  "canvasHash": "9aab768f"
}
```

该样本对应「Linux + 软件渲染（SwiftShader）+ 4 核」一类设备，验证了软件渲染器在
真实浏览器里确实以 `ANGLE (…, SwiftShader …)` 形式上报——这类渲染串同样存在于 apify
数据的 `videoCard` 分布中。

## 七、运行时更新与回退

- 更新：`bes update-fp` 会同时生成 `data/device_profiles.json`（`fpupdate.Update`）。
- 数据文件缺失 / 解析失败：`loadDeviceProfileStore` 返回 nil，引擎回退到旧的
  KB 独立采样器（行为兼容）。
- 不支持条件（iOS / Edge 等不在 supported 集合内的 UA）：同样回退旧路径。

## 八、已知限制

1. **canvas 实际 hash 值仍为合成**：联合分布让 `canvas key` 的 `(os, gpu vendor 档)`
   自洽，但 toDataURL 的像素值仍是确定性合成 PNG（无真实 GPU 逐机型 canvas 样本库）。
   真实 canvas hash 依赖更大规模自采/开源样本，属后续增量。
2. **时区是独立地理轴**：`network.json` 不含时区（fingerprint-generator 把 locale/geo
   单独注入），故时区↔语言仍走 KB 的配对；时区与其设备维度的相关性为弱关联，未建模。
3. **数据源自身有污染**：android UA 桶里混入少量桌面 `Direct3D11` 渲染器（模拟器/机器
   指纹），在 `buildStore` 时按 OS 一致性过滤并丢弃整个 GPU 分布都不兼容的 UA。