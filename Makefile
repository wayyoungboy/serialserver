# VSP Project Makefile
# 用法: make <target>

.PHONY: all clean build-server build-client build-assets build-windows build-installer package release dev-server dev-windows test test-e2e help

# 版本号
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "v0.1.0")
BUILD_DIR := build/release
GO_VERSION := 1.25

# 颜色输出
GREEN := \033[32m
YELLOW := \033[33m
RESET := \033[0m

all: build-server build-client build-windows

# ==================== 构建 ====================

build-server:
	@echo "$(GREEN)Building vsp-server...$(RESET)"
	@cd vsp-server && CGO_ENABLED=1 go build -ldflags="-s -w -X main.Version=$(VERSION)" -o ../$(BUILD_DIR)/vsp-server ./cmd

build-client:
	@echo "$(GREEN)Building V2 CLI clients for multiple platforms...$(RESET)"
	@mkdir -p $(BUILD_DIR)
	@# Linux amd64
	@cd vsp-client && GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-s -w" -o ../$(BUILD_DIR)/device-agent-v2-linux-amd64 ./cmd/device-agent-v2
	@cd vsp-client && GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-s -w" -o ../$(BUILD_DIR)/desktop-gateway-v2-linux-amd64 ./cmd/desktop-gateway-v2
	@# Linux arm64
	@cd vsp-client && GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -ldflags="-s -w" -o ../$(BUILD_DIR)/device-agent-v2-linux-arm64 ./cmd/device-agent-v2
	@cd vsp-client && GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -ldflags="-s -w" -o ../$(BUILD_DIR)/desktop-gateway-v2-linux-arm64 ./cmd/desktop-gateway-v2
	@# Windows amd64
	@cd vsp-client && GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-s -w" -o ../$(BUILD_DIR)/device-agent-v2-windows-amd64.exe ./cmd/device-agent-v2
	@cd vsp-client && GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-s -w" -o ../$(BUILD_DIR)/desktop-gateway-v2-windows-amd64.exe ./cmd/desktop-gateway-v2
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
	@# V2 CLI clients
	@cd $(BUILD_DIR) && gzip -k device-agent-v2-linux-amd64 -c > packages/device-agent-v2-$(VERSION)-linux-amd64.gz
	@cd $(BUILD_DIR) && gzip -k device-agent-v2-linux-arm64 -c > packages/device-agent-v2-$(VERSION)-linux-arm64.gz
	@cd $(BUILD_DIR) && zip packages/device-agent-v2-$(VERSION)-windows-amd64.zip device-agent-v2-windows-amd64.exe
	@cd $(BUILD_DIR) && gzip -k desktop-gateway-v2-linux-amd64 -c > packages/desktop-gateway-v2-$(VERSION)-linux-amd64.gz
	@cd $(BUILD_DIR) && gzip -k desktop-gateway-v2-linux-arm64 -c > packages/desktop-gateway-v2-$(VERSION)-linux-arm64.gz
	@cd $(BUILD_DIR) && zip packages/desktop-gateway-v2-$(VERSION)-windows-amd64.zip desktop-gateway-v2-windows-amd64.exe
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
	@rm -f vsp-client/device-agent-v2*
	@rm -f vsp-client/desktop-gateway-v2*

# ==================== 开发 ====================

dev-server:
	@cd vsp-server && go run ./cmd

dev-windows:
	@cd vsp-windows && wails dev

test:
	@cd vsp-server && go test ./...
	@cd vsp-client && go test ./...
	@cd vsp-windows && go test ./...
	@cd vsp-windows/frontend && npm run build
	@cd tests/e2e && go test ./...

test-e2e:
	@cd tests/e2e && go test ./...

# ==================== 帮助 ====================

help:
	@echo "VSP Project Build System"
	@echo ""
	@echo "Targets:"
	@echo "  make build-server    - Build vsp-server (Linux)"
	@echo "  make build-client    - Build device-agent-v2 and desktop-gateway-v2"
	@echo "  make build-assets    - Generate Windows app icon assets"
	@echo "  make build-windows   - Build VSPManager (Windows GUI)"
	@echo "  make build-installer - Build Windows NSIS installer"
	@echo "  make package         - Create release packages"
	@echo "  make release         - Show release instructions"
	@echo "  make clean           - Remove build artifacts"
	@echo "  make dev-server      - Run server in dev mode"
	@echo "  make dev-windows     - Run Windows client in dev mode"
	@echo "  make test            - Run integration tests"
	@echo "  make test-e2e        - Run Linux pseudo-terminal serial relay E2E"
	@echo ""
	@echo "Version: $(VERSION)"
