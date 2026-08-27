# 设计约束 — 来自实战踩坑

> 这些约束不是理论推导的，是每次补环境被检测/崩溃后用血泪换来的。
> 每条都标注了**踩坑场景**和**正确做法**。
>
> **架构备注：** 从 Node.js vm 迁移到 Go + v8go 后，部分约束被架构天然解决（标注 ⚡），
> 但其余约束仍然适用，必须在 Go 侧实现时遵守。

## 1. navigator 属性必须与请求 UA 版本一致

**踩坑场景：** 快照用了 Chrome 120 的 navigator，但请求 UA 写的 Chrome 131。目标 SDK 检查 `navigator.userAgent` 版本号与 UA header 是否匹配，不匹配 → 签名拒绝。

**正确做法：**
- 快照按 Chrome 大版本管理（`chrome-120.json`, `chrome-131.json`）
- 请求 UA 必须与快照版本严格一致
- `navigator.userAgentData.brands` 也要同步更新

## 2. document.URL / location.href 必须可配置

**踩坑场景：** 目标网站的 JS 读取 `document.URL` 和 `location.href` 进行校验，如果两者不可配置或不一致，签名计算会失败

**正确做法：**
- `document.URL` 和 `location.href` 必须在创建沙箱时可注入
- 两者必须保持一致（`document.URL === location.href`）
- `location.origin`, `location.pathname`, `location.search` 需从 href 自动解析

## 3. innerWidth / innerHeight 必须可配置

**踩坑场景：** 某些签名算法读取 `window.innerWidth` 参与哈希。默认值跟真实浏览器不一致 → 签名错误。

**正确做法：**
- 从快照读取默认值
- 允许运行时覆盖
- `window.outerWidth`, `screen.width` 等关联属性保持合理关系

## 4. top / parent / frames 不能用 Proxy

**踩坑场景：** 用 Proxy 包装 `window.top` 和 `window.parent` 模拟同源 frame。目标 SDK 做 `window.top === window` 检查时，Proxy 的身份比较失败 → 崩溃。

**正确做法：**
- `top` 和 `parent` 在无 frame 场景下直接指向 `window` 自身（`window.top === window`）
- `frames` 也指向 `window`（`window.frames === window`）
- 不用 Proxy，用直接引用赋值
- 如果需要模拟 iframe，用普通对象 + 显式属性

## 5. canary 探针属性必须正确返回 undefined

**踩坑场景：** 目标 SDK 检查 `navigator.pemrissions`（故意拼错 `permissions`）。在 mock 环境中如果用了 Proxy 的 `get` trap 返回空对象，SDK 认为环境异常 → 拒绝执行。

**正确做法：**
- 不存在的属性必须返回 `undefined`，不能返回空对象或抛错
- 不要用 `new Proxy(navigator, { get(target, key) { return target[key] ?? {} })` 这种写法
- 正确做法：只定义已知属性，其余走默认 prototype chain（自然返回 undefined）
- 检查清单（常见 canary）：
  - `navigator.pemrissions` → undefined ✓
  - `window.callPhantom` → undefined ✓
  - `window._phantom` → undefined ✓
  - `window.__nightmare` → undefined ✓
  - `window.domAutomation` → undefined ✓
  - `window.domAutomationController` → undefined ✓

## 6. Node 痕迹必须抹除

**踩坑场景：** 目标 SDK 检查 `typeof Buffer !== 'undefined'` 或 `typeof process !== 'undefined'`。Node 环境中这些是全局存在的 → 检测到非浏览器环境。

**正确做法：**
- vm.createContext 创建的新全局中默认没有 Buffer/process/require
- 但需显式确保：`typeof Buffer === 'undefined'`, `typeof process === 'undefined'`, `typeof require === 'undefined'`, `typeof global === 'undefined'`（用 `window` 代替）
- `__dirname`, `__filename`, `module` 也需清除

## 7. toString 必须返回原生格式

**踩坑场景：** 目标 SDK 检查 `navigator.toString()`。mock 对象返回 `"[object Object]"` 而非 `"[object Navigator]"` → 检测异常。

**正确做法：**
- `navigator.toString()` → `"[object Navigator]"`
- `screen.toString()` → `"[object Screen]"`
- `document.toString()` → `"[object HTMLDocument]"`
- `window.toString()` → `"[object Window]"`
- 函数的 `toString()` → `"function xxx() { [native code] }"` 格式
- 用 `Object.defineProperty` 定义 `Symbol.toStringTag`

## 8. 事件循环行为需可控

**踩坑场景：** 目标 SDK 使用 `setTimeout` 异步初始化，但 vm 上下文中的 setTimeout 行为与浏览器不一致，导致初始化未完成就读取了签名 → 空值。

**正确做法：**
- setTimeout/setInterval 在 vm 外（Node 主线程）注册，回调在 vm 内执行
- 提供 `await sandbox.flushTimers()` 等待所有待执行定时器完成
- `requestAnimationFrame` → setTimeout(16ms)
- 提供 `sandbox.runMicrotasks()` 手动 flush 微任务队列

## 9. cookie 读写需完整模拟

**踩坑场景：** 目标 SDK 写 cookie 后立即读取，但 mock 的 document.cookie 只支持写入不支持读取 → SDK 读不到刚写的 cookie。

**正确做法：**
- document.cookie 实现完整的 CookieStore：
  - setter：解析 `name=value; path=/; ...` 并存储
  - getter：返回 `name1=value1; name2=value2` 格式
  - 过期、path、domain 语义（最小实现）
- 与网络层联动：响应 Set-Cookie 自动写入 cookie store

## 10. Chrome 特征对象

**踩坑场景：** 真 Chrome 有 `window.chrome` 对象（含 `runtime`, `loadTimes`, `csi`, `app` 等）。缺少这个对象 → 部分检测认定为非 Chrome。

**正确做法：**
- `window.chrome` 从快照读取
- 至少包含：`{ runtime: {}, loadTimes: function(){...}, csi: function(){...} }`
- `window.chrome.runtime` 的 `onConnect`, `onMessage` 等为空对象
