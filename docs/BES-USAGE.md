# BES (browser-env-sandbox) 使用指南

> 通用 V8 沙箱 + TLS/HTTP/2 指纹引擎。零 Chrome/Node 依赖。
> 仓库: github.com/zninggo/browser-env-sandbox

## 一、启动

```bash
export PATH=$PATH:/usr/local/go/bin
cd browser-env-sandbox
go build -o bes-server ./cmd/bes-server
# 本地默认 8080；Docker/生产与 SDK 默认 19821（与 sdk/node、sdk/python 一致）
./bes-server --port 19821
# 默认无 auth，API 开放。生产环境加 --auth-token <token>
```

## 二、HTTP API 一览

| 方法 | 路径 | 用途 |
|------|------|------|
| GET | `/health` | 存活检查 |
| GET | `/api/session` | 列出所有 session |
| POST | `/api/session` | 创建 V8 session（自动加载 env_shim） |
| POST | `/api/session/{id}/eval` | 执行 JS 代码 |
| POST | `/api/session/{id}/script` | 加载具名脚本 |
| POST | `/api/session/{id}/call` | 调用全局函数 |
| GET | `/api/session/{id}/fingerprint` | 获取完整浏览器指纹 |
| GET | `/api/session/{id}/cookies` | 读取 cookie jar |
| POST | `/api/session/{id}/cookies` | 设置 cookie |
| DELETE | `/api/session/{id}` | 关闭 session |
| GET | `/api/session/{id}/stream/console` | SSE 控制台日志流 |
| GET | `/api/session/{id}/stream/network` | SSE 网络事件流 |

## 三、核心用法

### 1. 创建 session

```bash
curl -X POST http://localhost:19821/api/session \
  -H 'Content-Type: application/json' \
  -d '{"browser":"chrome","os":"windows"}'
# → {"session_id":"sess-xxx","fingerprint":{...}}
```

创建时自动加载 env_shim（navigator/window/document/canvas/WebGL 等 863 个全局构造函数）。

可选字段：
- `browser` / `os` — 指纹平台（chrome/windows、chrome/macos 等）
- `seed` — 指纹种子。**不传或 0 = 每次唯一随机**；传固定值（如 `42`）= 确定性复现同一指纹。响应中 `fingerprint.seed` 会带出生成时实际使用的 seed，记录即可复现
- `timezone` — 指定时区（如 `"Asia/Tokyo"`），用于与代理 IP 地理保持一致。`languages` 自动按时区联动（知识库配对）。不传 = 随机；未知时区名自动回退随机。当前支持：`Asia/Shanghai` / `Asia/Tokyo` / `Asia/Seoul` / `Asia/Singapore` / `America/New_York` / `Europe/London`
- `location` — 模拟的 URL（影响 document.location）
- `cookies` — 预设 cookie `{"name":"value"}`
- `proxy` — 代理 URL
- `net_mode` — 网络模式（live/replay）
- `preload` — 脚本文件路径数组，创建时自动加载
- `init` — JS 代码，创建后自动执行

### 2. 执行 JS

```bash
curl -X POST http://localhost:19821/api/session/sess-xxx/eval \
  -H 'Content-Type: application/json' \
  -d '{"code":"navigator.userAgent + \" / \" + navigator.platform"}'
# → {"result":"Mozilla/5.0 ... Chrome/131 ... / Win32"}
```

返回值始终是字符串。复杂对象需在 JS 侧 JSON.stringify。

### 3. 加载脚本

```bash
curl -X POST http://localhost:19821/api/session/sess-xxx/script \
  -H 'Content-Type: application/json' \
  -d '{"name":"mylib","content":"function add(a,b){return a+b}"}'
# → {}（成功无返回体）
```

加载后的脚本在 session 内全局可用，后续 eval/call 可直接引用。

### 4. 调用函数

```bash
curl -X POST http://localhost:19821/api/session/sess-xxx/call \
  -H 'Content-Type: application/json' \
  -d '{"function":"add","args":["1","2"]}'
# → {"result":"3"}
```

### 5. Cookie 管理

```bash
# 读
curl http://localhost:19821/api/session/sess-xxx/cookies

# 写
curl -X POST http://localhost:19821/api/session/sess-xxx/cookies \
  -H 'Content-Type: application/json' \
  -d '{"name":"session_id","value":"abc123","domain":"example.com"}'
```

## 四、TLS / HTTP/2 指纹层（Go SDK）

bes 内置 Chrome 精确指纹的 HTTP 客户端（`internal/netlayer`）：

```go
import "github.com/zninggo/bes/internal/netlayer"

client := netlayer.NewUTLSClient("chrome")
client.SetTimeout(30 * time.Second)

resp, err := client.Request("GET", "https://example.com/", headers, nil)
// resp.Status, resp.Body, resp.Headers, resp.Cookies
```

指纹对齐情况（commit dbe0344，用 tls.peet.ws 验证）：
- **TLS JA4**: `t13d1516h2_8daaf6152771_806a8c22fdea`（Chrome 151 逐字符相同）
- **HTTP/2 akamai hash**: `52d84b11737d980aef856699f885ca86`（Chrome 151 逐字符相同）
- SETTINGS: 4 entries（HEADER_TABLE_SIZE=65536, ENABLE_PUSH=0, INITIAL_WINDOW_SIZE=6291456, MAX_HEADER_LIST_SIZE=262144）
- WINDOW_UPDATE: 15663105（connection-level）
- HEADERS: Priority flag（exclusive, weight=256）
- 伪头顺序: `:method, :authority, :scheme, :path`
- 帧合并写入（1 个 TLS record）

## 五、Python 调用示例

```python
import json, urllib.request

BES = "http://localhost:19821"

def bes_api(path, data=None):
    url = f"{BES}{path}"
    if data:
        req = urllib.request.Request(url, data=json.dumps(data).encode(),
                                     headers={"Content-Type": "application/json"})
    else:
        req = urllib.request.Request(url)
    return json.loads(urllib.request.urlopen(req, timeout=30).read())

# 创建 session
sid = bes_api("/api/session", {"browser": "chrome", "os": "windows"})["session_id"]

# 执行 JS
result = bes_api(f"/api/session/{sid}/eval", {"code": "navigator.userAgent"})["result"]

# 加载脚本
with open("sdk.js") as f:
    bes_api(f"/api/session/{sid}/script", {"name": "sdk", "content": f.read()})

# 调用
sig = bes_api(f"/api/session/{sid}/eval",
              {"code": "sdk.sign('nonce_value')"})["result"]
```

## 六、架构边界

- **bes 提供通用能力**：V8 JS 执行 + TLS/HTTP/2 指纹 + 网络请求 + cookie 管理
- **不包含任何站点专属逻辑**：页面自有逻辑、业务签名、特定 cookie 流程等由调用方负责
- **零外部浏览器依赖**：不依赖 Chrome/Chromium/Puppeteer/Node

## 七、验证指纹

```bash
# 用 bes netlayer 客户端访问 tls.peet.ws
go run ./cmd/h2probe  # （需自建探针）
# 或用 curl_cffi（Python，走相同指纹）
python3 -c "
from curl_cffi import requests
r = requests.get('https://tls.peet.ws/api/all', impersonate='chrome')
d = r.json()
print('JA4:', d['tls']['ja4'])
print('H2:', d['http2']['akamai_fingerprint_hash'])
"
# 期望: JA4=...806a8c22fdea, H2=52d84b...
```
