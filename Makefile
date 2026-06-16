.PHONY: dev dev-frontend dev-backend build frontend release release-snapshot clean help

# 版本信息（构建时自动注入）
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
DATE    ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS := -s -w \
	-X github.com/anthropic/airesourcemanager/cmd.Version=$(VERSION) \
	-X github.com/anthropic/airesourcemanager/cmd.Commit=$(COMMIT) \
	-X github.com/anthropic/airesourcemanager/cmd.Date=$(DATE)

# 帮助信息
help:
	@echo "可用命令:"
	@echo "  make dev              - 开发模式：同时启动前后端"
	@echo "  make dev-frontend     - 仅启动前端开发服务器"
	@echo "  make dev-backend      - 仅启动后端"
	@echo "  make build            - 生产构建：前端 + 单二进制（当前平台）"
	@echo "  make release-snapshot - GoReleaser 本地快照（全平台，不发布）"
	@echo "  make release          - GoReleaser 正式发布（需 git tag）"
	@echo "  make clean            - 清理构建产物"

# --- 开发 ---

dev:
	@echo "启动开发模式..."
	@cd web && npm run dev &
	@go run . serve --no-open

dev-frontend:
	cd web && npm run dev

dev-backend:
	go run . serve

# --- 构建 ---

frontend:
	@echo "构建前端..."
	@cd web && npm install && npm run build

build: frontend
	@echo "构建后端..."
	@CGO_ENABLED=1 go build -ldflags "$(LDFLAGS)" -tags sqlite_json -o dist/aimanager .
	@echo "构建完成: dist/aimanager"

# --- 发布（GoReleaser） ---

# 本地快照构建：全平台交叉编译，不发布，不需要 git tag
release-snapshot: frontend
	goreleaser release --snapshot --clean

# 正式发布：需要 git tag + GITHUB_TOKEN
release: frontend
	goreleaser release --clean

# --- 清理 ---

clean:
	@rm -rf dist/
	@rm -rf web/dist/
	@rm -f aimanager
	@echo "清理完成"
