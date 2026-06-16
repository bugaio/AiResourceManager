# AiResourceManager

统一管理 AI 项目资源的本地工具。单二进制部署，内嵌 Web UI，支持 Skill / MCP / SubAgent 三类资源的配置管理、分组、部署和追踪。

## 功能

- **Skill 管理** — 创建/编辑 Skill 配置，通过软链接部署到目标目录
- **MCP 管理** — JSONC 编辑器，DeepMerge 部署到目标 JSON 文件，冲突检测（Union-Find 分组）
- **SubAgent 管理** — SubAgent 配置管理，软链接部署
- **分组** — 资源分组，支持跟踪模式（自动同步部署变更）
- **路径别名** — 目标路径别名管理，按资源类型隔离
- **单二进制** — Go embed 内嵌前端，开箱即用

## 技术栈

| 层 | 技术 |
|---|---|
| 后端 | Go, Gin, SQLite (CGO), Cobra, Viper, Zap |
| 前端 | Vue 3, TypeScript, Element Plus, Tailwind CSS, Monaco Editor |
| 构建 | GoReleaser, Zig (交叉编译 C 工具链) |

## 快速开始

```bash
# 下载对应平台的 release 包解压后直接运行
./aimanager serve

# 默认监听 http://localhost:3678
# 可指定端口
./aimanager serve -p 8080
```

## 开发

### 环境要求

- Go 1.25+
- Node.js 18+ / npm
- SQLite (系统自带或通过 CGO 编译)

### 本地开发

```bash
# 同时启动前后端（前端 :5173 + 后端 :3678）
make dev

# 或分开启动
make dev-frontend  # 终端 1
make dev-backend   # 终端 2
```

### 本地构建（当前平台）

```bash
make build
# 产物: dist/aimanager
```

## 打包发布

项目使用 [GoReleaser](https://goreleaser.com/) 进行跨平台打包。

### 前置依赖

```bash
# macOS
brew install goreleaser zig

# 或手动安装:
# goreleaser: https://goreleaser.com/install/
# zig: https://ziglang.org/download/
```

> **为什么需要 Zig？** 项目依赖 `mattn/go-sqlite3`（CGO），跨平台编译需要 C 交叉编译器。Zig 提供零配置的交叉编译工具链，替代传统的 `gcc-aarch64` / `mingw-w64` 等。

### 支持平台

| 平台 | 架构 | 格式 |
|---|---|---|
| macOS | arm64 (Apple Silicon) | tar.gz |
| macOS | amd64 (Intel) | tar.gz |
| Windows | arm64 | zip |
| Windows | amd64 | zip |

### 本地快照构建（全平台）

不需要 git tag，不发布到 GitHub，仅本地生成全平台二进制：

```bash
make release-snapshot
# 产物目录: dist/
```

### 正式发布

```bash
# 1. 打 tag
git tag -a v0.1.0 -m "first release"
git push origin v0.1.0

# 2. 发布到 GitHub Releases（需要 GITHUB_TOKEN）
export GITHUB_TOKEN=your_token
make release
```

### GoReleaser 配置说明

配置文件: `.goreleaser.yaml`

- 构建前自动执行 `npm run build` 生成前端资源
- 使用 `zig cc` 作为交叉编译器处理 CGO
- 通过 ldflags 注入版本号、commit、构建时间
- Windows 产物自动打包为 zip，其余为 tar.gz
- 生成 checksums.txt 校验文件

## 项目结构

```
├── main.go              # 入口
├── embed.go             # go:embed 前端资源 + 配置
├── cmd/                 # CLI 命令（cobra）
├── internal/
│   ├── handler/         # HTTP 路由处理
│   ├── service/         # 业务逻辑
│   ├── repo/            # 数据库操作 + 迁移
│   ├── model/           # 数据模型
│   ├── util/            # 工具函数（DeepMerge 等）
│   └── config/          # 配置加载
├── web/                 # Vue 3 前端
│   ├── src/
│   │   ├── components/  # 组件（deploy/editor/sidebar）
│   │   ├── stores/      # Pinia 状态
│   │   ├── api/         # API 调用层
│   │   └── types/       # TypeScript 类型
│   └── dist/            # 构建产物（git ignored）
├── configs/             # 默认配置文件
├── .goreleaser.yaml     # GoReleaser 配置
└── Makefile             # 开发 & 构建命令
```

## 命令参考

```bash
aimanager serve           # 启动服务（默认 :3678）
aimanager serve -p 8080   # 指定端口
aimanager serve --no-open # 不自动打开浏览器
aimanager version         # 显示版本信息
```

## License

MIT
