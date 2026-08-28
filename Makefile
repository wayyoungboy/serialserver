# VSP Project Makefile
# 用法: make <target>

.PHONY: all clean build-server build-client build-assets build-windows build-installer package release dev-server dev-windows test test-e2e smoke help

# 版本号
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "v0.1.0")
BUILD_DIR := build/release
GO_VERSION := 1.25

# 颜色输出
GREEN := \033[32m
YELLOW := \033[33m
RESET := \033[0m

all: build-server build-client

# ==================== 构建 ====================

build-server:
	@echo "$(GREEN)Building vsp-server...$(RESET)"
	@mkdir -p $(BUILD_DIR)
	@cd vsp-server && CGO_ENABLED=0 go build -ldflags="-s -w -X main.Version=$(VERSION)" -o ../$(BUILD_DIR)/vsp-server ./cmd

build-client:
	@echo "$(GREEN)Building CLI clients for multiple platforms...$(RESET)"
	@mkdir -p $(BUILD_DIR)
	@# Linux amd64
	@cd vsp-client && GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-s -w" -o ../$(BUILD_DIR)/device-agent-linux-amd64 ./cmd/device-agent
	@cd vsp-client && GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-s -w" -o ../$(BUILD_DIR)/desktop-gateway-linux-amd64 ./cmd/desktop-gateway
	@# Linux arm64
	@cd vsp-client && GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -ldflags="-s -w" -o ../$(BUILD_DIR)/device-agent-linux-arm64 ./cmd/device-agent
	@cd vsp-client && GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -ldflags="-s -w" -o ../$(BUILD_DIR)/desktop-gateway-linux-arm64 ./cmd/desktop-gateway
	@# Windows amd64
	@cd vsp-client && GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-s -w" -o ../$(BUILD_DIR)/device-agent-windows-amd64.exe ./cmd/device-agent
	@cd vsp-client && GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-s -w" -o ../$(BUILD_DIR)/desktop-gateway-windows-amd64.exe ./cmd/desktop-gateway
	@echo "$(YELLOW)macOS CLI artifacts are built by the GitHub Actions macOS runner because the serial dependency needs Darwin build tags.$(RESET)"

build-assets:
	@echo "$(GREEN)Generating Windows app icons...$(RESET)"
	@cd vsp-windows && go run tools/gen_windows_assets.go

build-windows: build-assets
	@echo "$(GREEN)Building VSPManager (Windows GUI)...$(RESET)"
	@cd vsp-windows && wails build -clean
	@mkdir -p $(BUILD_DIR)
	@cp -r vsp-windows/build/bin/VSPManager.exe $(BUILD_DIR)/

build-installer: build-windows
	@echo "$(GREEN)Building Windows installer with NSIS...$(RESET)"
	@command -v makensis >/dev/null 2>&1 || { echo "$(YELLOW)NSIS makensis is required to build the installer.$(RESET)"; exit 1; }
	@cd vsp-windows && mkdir -p build/installer && makensis "/DAPP_VERSION=$(VERSION)" packaging/windows/VSPManager.nsi
	@mkdir -p $(BUILD_DIR)
	@cp vsp-windows/build/installer/*.exe $(BUILD_DIR)/

# ==================== 打包 ====================

package: build-server build-client build-installer
	@echo "$(GREEN)Creating release packages...$(RESET)"
	@mkdir -p $(BUILD_DIR)/packages
	@# Windows 客户端完整包
	@cd $(BUILD_DIR) && zip -r packages/VSPManager-$(VERSION)-windows-amd64.zip VSPManager.exe
	@cp $(BUILD_DIR)/VSPManager-*-Setup.exe $(BUILD_DIR)/packages/
	@# Linux 服务端
	@cd $(BUILD_DIR) && gzip -k vsp-server -c > packages/vsp-server-$(VERSION)-linux-amd64.gz
	@# CLI clients
	@cd $(BUILD_DIR) && gzip -k device-agent-linux-amd64 -c > packages/device-agent-$(VERSION)-linux-amd64.gz
	@cd $(BUILD_DIR) && gzip -k device-agent-linux-arm64 -c > packages/device-agent-$(VERSION)-linux-arm64.gz
	@cd $(BUILD_DIR) && zip packages/device-agent-$(VERSION)-windows-amd64.zip device-agent-windows-amd64.exe
	@cd $(BUILD_DIR) && gzip -k desktop-gateway-linux-amd64 -c > packages/desktop-gateway-$(VERSION)-linux-amd64.gz
	@cd $(BUILD_DIR) && gzip -k desktop-gateway-linux-arm64 -c > packages/desktop-gateway-$(VERSION)-linux-arm64.gz
	@cd $(BUILD_DIR) && zip packages/desktop-gateway-$(VERSION)-windows-amd64.zip desktop-gateway-windows-amd64.exe
	@echo "$(GREEN)Packages created in $(BUILD_DIR)/packages/$(RESET)"

# ==================== 发布 ====================

release: package
	@echo "$(YELLOW)To create a GitHub release:$(RESET)"
	@echo "  1. git tag $(VERSION)"
	@echo "  2. git push origin $(VERSION)"
	@echo ""
	@echo "Or use gh CLI:"
	@echo "  gh release create $(VERSION) $(BUILD_DIR)/packages/* --title 'VSP $(VERSION)' --notes 'Release notes here'"

# ==================== 清理 ====================

clean:
	@echo "$(GREEN)Cleaning build artifacts...$(RESET)"
	@rm -rf $(BUILD_DIR)
	@rm -rf vsp-windows/build/bin
	@rm -f vsp-server/vsp-server
	@rm -f vsp-client/device-agent*
	@rm -f vsp-client/desktop-gateway*

# ==================== 开发 ====================

dev-server:
	@cd vsp-server && go run ./cmd

dev-windows:
	@cd vsp-windows && wails dev

test:
	@cd vsp-server && go test ./...
	@cd vsp-client && go test ./...
	@cd vsp-windows && go test ./...
	@cd vsp-windows/frontend && npm ci && npm run build
	@cd tests/e2e && go test ./...

test-e2e:
	@cd tests/e2e && go test ./...

smoke:
	@echo "$(GREEN)Building current-platform binaries and printing CLI help...$(RESET)"
	@cd vsp-server && go build -o vsp-server ./cmd
	@cd vsp-client && go build -o device-agent ./cmd/device-agent
	@cd vsp-client && go build -o desktop-gateway ./cmd/desktop-gateway
	@echo ""
	@echo "$(YELLOW)device-agent help$(RESET)"
	@vsp-client/device-agent -h
	@echo ""
	@echo "$(YELLOW)desktop-gateway help$(RESET)"
	@vsp-client/desktop-gateway -h
	@echo ""
	@echo "$(GREEN)Binaries: vsp-server/vsp-server vsp-client/device-agent vsp-client/desktop-gateway$(RESET)"
	@echo "$(YELLOW)Run the server from vsp-server/ so configs/ and web/dist resolve.$(RESET)"
	@echo "$(YELLOW)Linux no-hardware relay smoke: make test-e2e$(RESET)"

# ==================== 帮助 ====================

help:
	@echo "VSP Project Build System"
	@echo ""
	@echo "Targets:"
	@echo "  make build-server    - Build vsp-server (Linux)"
	@echo "  make build-client    - Build device-agent and desktop-gateway"
	@echo "  make build-assets    - Generate Windows app icon assets"
	@echo "  make build-windows   - Build VSPManager (Windows GUI)"
	@echo "  make build-installer - Build Windows NSIS installer"
	@echo "  make package         - Create release packages"
	@echo "  make release         - Show release instructions"
	@echo "  make clean           - Remove build artifacts"
	@echo "  make dev-server      - Run server in dev mode"
	@echo "  make dev-windows     - Run Windows client in dev mode"
	@echo "  make all             - Build server + CLI (no Windows GUI / Wails)"
	@echo "  make smoke           - Build local binaries and print CLI help"
	@echo "  make test            - Run Go tests, frontend build, and Linux E2E"
	@echo "  make test-e2e        - Run Linux pseudo-terminal serial relay E2E"
	@echo ""
	@echo "Version: $(VERSION)"
