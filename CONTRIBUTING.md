# 贡献指南

感谢你对 browser-env-sandbox 的兴趣！本文说明如何搭建开发环境、提交代码与发布版本。

## 行为准则

参与本项目即代表你同意遵守 [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md)。请在所有交流中保持尊重与专业。

## 能力边界（重要）

browser-env-sandbox 是**通用浏览器环境模拟引擎**，只提供：

- 纯 V8 Isolate 执行浏览器 JS
- 程序化自洽指纹生成
- TLS/HTTP/2 帧级指纹
- JSON-over-HTTP 桥接 + 多语言 SDK

**不接受网站特定的补环境脚本或指纹数据集进入主仓库。** 网站专属逻辑应通过 `preload` / `script` 端点从外部注入，或放入独立的 `bes-site-profiles` 仓库。详见 [docs/open-source-survey.md](docs/open-source-survey.md) 第 8 章。

## 前置要求

| 工具 | 版本 | 说明 |
|------|------|------|
| Go | 1.25+ | 模块路径 `github.com/zninggo/browser-env-sandbox` |
| C/C++ 工具链 | — | CGO 必需，v8go 通过 CGO 调用 V8 静态库 |
| lld | 19+ | v8go 静态库链接需要 `lld`（`-fuse-ld=lld`），Linux 上安装 `lld-19` |
| Git | 2.20+ | — |

> **v8go 为私有 fork**：本项目使用 `github.com/zninggo/v8go`（自行 fork+build），拉取前需配置 `GOPRIVATE`（见下方本地编译）。如果你无权访问该私有仓库，请通过 [Docker](#docker-开发) 构建。

> **Windows 用户注意**：v8go 的预编译 V8 静态库仅支持 **Linux (x86_64/arm64) 和 macOS**，不支持 Windows 原生编译。请通过 [Docker](#docker-开发) 或 WSL2 运行。

## 获取源码

```bash
git clone https://github.com/zninggo/browser-env-sandbox.git
cd browser-env-sandbox
```

## 本地编译

CGO 必须开启。v8go 为私有 fork，首次编译前需配置环境变量（与 CI 一致）：

```bash
# 环境变量（v8go 私有拉取 + lld 链接器）
export GOPRIVATE=github.com/zninggo/v8go
export GOSUMDB=off
export CGO_LDFLAGS_ALLOW="-fuse-ld=.*"

# Linux 上安装 lld-19（v8go 静态库链接需要）
sudo apt-get install -y lld-19
sudo update-alternatives --install /usr/bin/ld.lld ld.lld /usr/bin/ld.lld-19 50

# 编译 CLI 与服务
CGO_ENABLED=1 go build -o bes ./cmd/bes
CGO_ENABLED=1 go build -o bes-server ./cmd/bes-server

# 编译并运行自测套件（浏览器环境检测，项数以 ./bes-selftest 实际输出为准）
CGO_ENABLED=1 go build -o bes-selftest ./cmd/bes-selftest
./bes-selftest

# 性能基准（对比 perf-baseline.json）
CGO_ENABLED=1 go build -o bes-bench ./cmd/bes-bench
./bes-bench --compare perf-baseline.json
```

编译通过后，按 [README.md](README.md) 的快速开始运行。

## Docker 开发

适合 Windows 用户或想要一致构建环境的贡献者：

```bash
docker-compose up -d    # API: 19821, CDP: 9223
```

## 提交规范

本项目采用 [约定式提交](https://www.conventionalcommits.org/zh-hans/)。每个提交信息以类型前缀开头：

```
<type>(<scope>): <subject>

<body 可选>
```

常用类型：

| type | 用途 |
|------|------|
| `feat` | 新功能 |
| `fix` | Bug 修复 |
| `docs` | 文档变更 |
| `refactor` | 重构（不改变行为） |
| `perf` | 性能优化 |
| `test` | 测试相关 |
| `chore` | 构建、依赖、CI 等杂项 |

示例：

```
feat(fpengine): 支持 Chrome 153 指纹知识库
fix(netlayer): 修复 H2 帧合并丢失 priority 标志
docs: 补充 Worker 执行能力说明
```

## Pull Request 流程

1. 从 `main` 拉取最新代码，新建分支：`git checkout -b feat/your-feature`
2. 保持改动聚焦——一个 PR 只解决一件事
3. 本地验证（需先按 [本地编译](#本地编译) 配置 `GOPRIVATE` / `GOSUMDB` / `CGO_LDFLAGS_ALLOW` 环境变量与 lld）：
   ```bash
   go vet ./...
   CGO_ENABLED=1 go build ./...
   ./bes-selftest
   ```
4. 提交 PR，标题遵循约定式提交格式
5. CI 会自动跑 `go vet` + `go build` + `bes-selftest`；全部通过后再请求评审
6. 评审通过后合并到 `main`

> 本项目目前没有 `_test.go` 单元测试，CI 以 `go vet` + `go build` + `bes-selftest` 为准。如果你新增了 `_test.go`，CI 会一并跑 `go test`（届时请同步更新本文档与 `.github/workflows/ci.yml`）。

## 安全漏洞

**不要通过公开 issue 或 PR 报告安全漏洞。** 请按 [SECURITY.md](SECURITY.md) 的私密渠道上报。

## License

提交代码即代表你同意以 [MIT License](LICENSE) 发布你的贡献。
