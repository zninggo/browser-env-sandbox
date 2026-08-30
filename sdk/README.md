# bes SDK

轻量客户端 SDK,通过 JSON-RPC over HTTP 连接 bes-server。支持 Go / Node.js / Python 三种语言。

## 服务器地址与 scheme

三 SDK 的 `serverAddr` 参数统一按以下规则解析 scheme:

| 传入值 | 解析结果 | 说明 |
| --- | --- | --- |
| `host:port`(如 `localhost:19821`) | `http://host:port` | 向后兼容默认值,用于本地 bes-server |
| `http://host:port` | 原样使用 | 显式 HTTP |
| `https://host[:port]` | 原样使用 | 连接 TLS 前置的线上实例 |

即:**若地址已含 `http://` 或 `https://` 前缀则直用,否则默认按 `http://` 拼接**。无需额外参数即可连接 HTTPS 服务器。

### 连接 HTTPS 服务器示例

线上实例(如经反向代理终止 TLS 的 `https://bes.zsso.de`)只需传入带 `https://` 前缀的地址。

**Go**
```go
import "github.com/zninggo/browser-env-sandbox/sdk/go/bes"

// 连接 HTTPS 线上实例
sandbox := bes.New("https://bes.zsso.de")
if err := sandbox.CreateSession(bes.SessionOptions{Browser: "chrome", OS: "windows"}); err != nil {
    log.Fatal(err)
}
defer sandbox.Close()

// 本地实例(向后兼容,无 scheme 默认 http://)
local := bes.New("localhost:19821")
```

**Node.js**
```js
const { Sandbox } = require('./sdk/node/bes');

// 连接 HTTPS 线上实例
const sandbox = await Sandbox.create({ serverAddr: 'https://bes.zsso.de' });
await sandbox.eval('navigator.userAgent');
await sandbox.close();

// 本地实例(向后兼容,无 scheme 默认 http://)
const local = await Sandbox.create({ serverAddr: 'localhost:19821' });
```

**Python**
```python
from bes import Sandbox

# 连接 HTTPS 线上实例
sandbox = Sandbox(server_addr="https://bes.zsso.de", browser="chrome", os="windows")
print(sandbox.eval("navigator.userAgent"))
sandbox.close()

# 本地实例(向后兼容,无 scheme 默认 http://)
local = Sandbox(server_addr="localhost:19821")
```

## 运行冒烟测试

scheme 解析契约的冒烟测试不依赖真实 bes-server,纯函数级验证:

```sh
# Go(仓库根目录)
go test ./sdk/go/...

# Node.js
node sdk/node/bes_scheme_test.js

# Python
python sdk/python/test_scheme.py
```
