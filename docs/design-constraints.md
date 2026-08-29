# 设计约束 — 浏览器环境模拟实现规范

> 本文档是 browser-env-sandbox 在实现浏览器 API 模拟时沉淀的技术规范。
> 每条都标注了**常见问题**和**正确做法**，可直接作为环境注入层的开发检查清单。
>
> **架构备注：** 基于 Go + v8go 的纯 V8 Isolate 天然满足部分约束（标注 ⚡），
> 其余约束在 Go 侧实现时仍然适用。
>
> **更新：** 2026-08-29 第二轮修复后新增第 11-14 条（V8 线程安全 / TypedArray 字节 / 原型链 / 回调返回值）。

## 1. navigator 属性必须与 UA 版本一致

**常见问题：** 模拟环境的 navigator 属性来自 Chrome 120 的数据，但请求 UA 写的 Chrome 131。依赖 `navigator.userAgent` 版本号与 UA header 一致性的逻辑会校验失败。

**正确做法：**
- 环境数据按 Chrome 大版本管理（`chrome-120.json`, `chrome-131.json`）
- 请求 UA 与环境数据版本严格一致
- `navigator.userAgentData.brands` 同步更新

## 2. document.URL / location.href 必须可配置

**常见问题：** 页面 JS 常读取 `document.URL` 和 `location.href` 并校验两者一致；模拟环境若不可配置或不一致，依赖该值的逻辑会失败。

**正确做法：**
- `document.URL` 和 `location.href` 在创建沙箱时可注入
- 两者保持一致（`document.URL === location.href`）
- `location.origin`, `location.pathname`, `location.search` 从 href 自动解析

## 3. innerWidth / innerHeight 必须可配置

**常见问题：** 部分页面逻辑读取 `window.innerWidth` 参与计算。默认值与真实浏览器不一致会导致结果偏差。

**正确做法：**
- 从环境数据读取默认值
- 允许运行时覆盖
- `window.outerWidth`, `screen.width` 等关联属性保持合理关系

## 4. top / parent / frames 不能用 Proxy ⚡

**常见问题：** 用 Proxy 包装 `window.top` 和 `window.parent` 模拟同源 frame。页面做 `window.top === window` 身份比较时，Proxy 的身份比较失败。

**正确做法：**
- `top` 和 `parent` 在无 frame 场景下直接指向 `window` 自身（`window.top === window`）
- `frames` 也指向 `window`（`window.frames === window`）
- 不用 Proxy，用直接引用赋值
- 如需模拟 iframe，用普通对象 + 显式属性

## 5. 未定义属性必须返回 undefined

**常见问题：** 页面代码可能访问任意属性（包括故意拼错的属性名）探测环境。若模拟层用 Proxy 的 `get` trap 对所有缺失属性返回空对象，会被识别为异常环境。

**正确做法：**
- 不存在的属性返回 `undefined`，不返回空对象也不抛错
- 不要用 `new Proxy(navigator, { get(target, key) { return target[key] ?? {} } })` 这种写法
- 只定义已知属性，其余走默认 prototype chain（自然返回 undefined）

## 6. 宿主痕迹必须为空 ⚡

**常见问题：** 在 Node.js `vm` 中模拟浏览器时，`Buffer`/`process`/`require` 等宿主全局对象会泄漏进沙箱。

**正确做法：**
- v8go Isolate 天然没有 Buffer/process/require——不是"设为 undefined"而是"从不存在"
- 自测断言：`typeof Buffer === 'undefined'` 等 6 项（见 bes-selftest）

## 7. toString 必须返回原生格式

**常见问题：** 模拟对象的 `toString()` 返回 `"[object Object]"` 而非原生标签，与真实浏览器行为不符。

**正确做法：**
- `navigator.toString()` → `"[object Navigator]"`
- `screen.toString()` → `"[object Screen]"`
- `document.toString()` → `"[object HTMLDocument]"`
- `window.toString()` → `"[object Window]"`
- 函数的 `toString()` → `"function xxx() { [native code] }"` 格式
- 用 `Object.defineProperty` 定义 `Symbol.toStringTag`

## 8. 事件循环行为需可控

**常见问题：** 页面 JS 用 `setTimeout` 异步初始化；若模拟环境的定时器行为与浏览器不一致，初始化未完成时读取相关值会得到空值。

**正确做法：**
- setTimeout/setInterval 在宿主侧（Go）注册，回调在沙箱内执行
- 提供 `FlushTimers` 等待所有待执行定时器完成
- `requestAnimationFrame` → setTimeout(16ms)
- 提供 `PerformMicrotasks` 手动 flush 微任务队列

## 9. cookie 读写需完整模拟

**常见问题：** 页面 JS 写 cookie 后立即读取；模拟的 `document.cookie` 若只支持写不支持读，会返回空。

**正确做法：**
- `document.cookie` 实现完整的 CookieStore：
  - setter：解析 `name=value; path=/; ...` 并存储
  - getter：返回 `name1=value1; name2=value2` 格式
  - 过期、path、domain 语义（最小实现）
- 与网络层联动：响应 Set-Cookie 自动写入 cookie store

## 10. Chrome 特征对象

**常见问题：** 真 Chrome 有 `window.chrome` 对象（含 `runtime`, `loadTimes`, `csi`, `app` 等）。模拟环境缺少它时，依赖该对象的逻辑会失败。

**正确做法：**
- `window.chrome` 从环境数据读取
- 至少包含：`{ runtime: {}, loadTimes: function(){...}, csi: function(){...} }`
- `window.chrome.runtime` 的 `onConnect`, `onMessage` 等为空对象

## 11. V8 调用必须留在 Isolate 线程

**常见问题：** v8go 的所有 Value/Object/Function API（`AsFunction()`、`Global()`、`Call()` 等）都不是线程安全的。在 Go goroutine 里直接调它们，与主线程 Eval 并发时可能随机崩溃（非确定性，压测才暴露）。

**正确做法：**
- 回调同步入口（V8 线程内）先把 `*v8go.Value` 转成具体类型（如 `AsFunction`）、保存 `ctx.Global()`，作为普通 Go 值传给 goroutine
- goroutine 内只做纯 Go 工作（网络、解析），绝不碰 v8go API
- 结果回传用 `scheduleTimer(0, ...)` 排回 Isolate 线程，在 timer 回调里执行 `cb.Call`
- 验证方式：并发 50+ 次 eval + async fetch 压测无崩溃

## 12. v8go 无法直接读写 TypedArray 字节

**常见问题：** `crypto.getRandomValues`、XHR arrayBuffer 等需要填充 TypedArray 的场景，v8go Value API 没有 `Bytes()`/`SetBytes()` 接口（只有 SharedArrayBuffer 有 backing store 访问）。

**正确做法：**
- Go 侧生成/处理字节后以 base64 字符串跨边界传输（`__besRandomBytes` 模式）
- JS 侧用 std-alphabet atob 解码填充（注意 shim part 加载顺序，part3 早于 fetch shim，需内联 atob 而不能引用 `__besB64ToUint8Array`）
- 规格：`getRandomValues` 上限 65536 字节，超出抛 `RangeError`

## 13. 模板实例不继承 JS 原型链

**常见问题：** Go 侧 ObjectTemplate/FunctionTemplate 创建的实例（`new Event()`），其 `[[Prototype]]` 是 `Object.prototype`，不是 `window.Event.prototype`——在 JS 侧往 `Event.prototype` 挂方法对模板实例无效。

**正确做法：**
- 方案 A（推荐）：JS 侧用包装函数替换全局构造器——调 Go 构造器造实例，`delete` 实例上的自有 noop 方法（避免遮蔽），再 `Object.setPrototypeOf(inst, Wrapper.prototype)` 挂上带状态的 JS 方法
- 方案 B：模板阶段直接在实例模板上挂方法（适合无状态 noop，无法维护 `defaultPrevented` 这类实例状态）
- 子类构造器给实例赋 `this[Symbol.toStringTag]` 时，父原型上的 tag 必须 `writable: true`，否则沿原型链赋值抛 TypeError
- 包装后实例 `Object.prototype.toString.call(new Event('x'))` 仍应为 `[object Event]`

## 14. v8go 回调返回值只能是基本类型

**常见问题：** Go FunctionCallback 里 `v8go.NewValue(iso, jsonString)` 返回的是 **JS 字符串**，不是解析后的对象。调用方期望 `measureText(...).width` 这类对象访问时会拿到 undefined。

**正确做法：**
- 需要返回真 JS 对象时，在 JS 侧包一层解析（如 part3 的 `ctx.measureText` 包装：`typeof m === 'string'` 时 `JSON.parse`）
- 或参考 `getSupportedExtensions` 用 `info.Context().RunScript("[...]")` 直接构造数组
- 对外契约（TextMetrics、getBoundingClientRect 等）必须在 JS 侧保证返回对象
