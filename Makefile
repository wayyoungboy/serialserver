# VSP Project Makefile
# 用法: make <target>

.PONES: all clean build-server build-client build-windows package release

# 版本号
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "v0.1.0")
BUILD_DIR := build/release
GO_VERSION := 1.25

# 颜色输出
GREEN := \033[32m
YELLOW := \033[33m
RESET := \033[0m

all: build-server build-client build-windows package

# ==================== 构建 ====================

build-server:
	@echo "$(GREEN)Building vsp-server...$(RESET)"
	@cd vsp-server && CGO_ENABLED=1 go build -ldflags="-s -w -X main.Version=$(VERSION)" -o ../$(BUILD_DIR)/vsp-server ./cmd

build-client:
	@echo "$(GREEN)Building device-client for multiple platforms...$(RESET)"
	@mkdir -p $(BUILD_DIR)
	@# Linux amd64
	@cd vsp-client && GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-s -w" -o ../$(BUILD_DIR)/device-client-linux-amd64 ./cmd/device-client
	@# Linux arm64
	@cd vsp-client && GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -ldflags="-s -w" -o ../$(BUILD_DIR)/device-client-linux-arm64 ./cmd/device-client
	@# Windows amd64
	@cd vsp-client && GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-s -w" -o ../$(BUILD_DIR)/device-client-windows-amd64.exe ./cmd/device-client
	@# macOS amd64
	@cd vsp-client && GOOS=darwin GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-s -w" -o ../$(BUILD_DIR)/device-client-darwin-amd64 ./cmd/device-client
	@# macOS arm64
	@cd vsp-client && GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 go build -ldflags="-s -w" -o ../$(BUILD_DIR)/device-client-darwin-arm64 ./cmd/device-client

build-windows:
	@echo "$(GREEN)Building VSPManager (Windows GUI)...$(RESET)"
	@cd vsp-windows && wails build -clean
	@mkdir -p $(BUILD_DIR)
	@cp -r vsp-windows/build/bin/VSPManager.exe $(BUILD_DIR)/
	@cp -r com0com $(BUILD_DIR)/com0com

# ==================== 打包 ====================

package: build-windows
	@echo "$(GREEN)Creating release packages...$(RESET)"
	@mkdir -p $(BUILD_DIR)/packages
	@# Windows 客户端完整包
	@cd $(BUILD_DIR) && zip -r packages/VSPManager-$(VERSION)-windows-amd64.zip VSPManager.exe com0com/
	@# Linux 服务端
	@cd $(BUILD_DIR) && gzip -k vsp-server -c > packages/vsp-server-$(VERSION)-linux-amd64.gz
	@# Device Client 各平台
	@cd $(BUILD_DIR) && gzip -k device-client-linux-amd64 -c > packages/device-client-$(VERSION)-linux-amd64.gz
	@cd $(BUILD_DIR) && gzip -k device-client-linux-arm64 -c > packages/device-client-$(VERSION)-linux-arm64.gz
	@cd $(BUILD_DIR) && zip packages/device-client-$(VERSION)-windows-amd64.zip device-client-windows-amd64.exe
	@cd $(BUILD_DIR) && gzip -k device-client-darwin-amd64 -c > packages/device-client-$(VERSION)-darwin-amd64.gz
	@cd $(BUILD_DIR) && gzip -k device-client-darwin-arm64 -c > packages/device-client-$(VERSION)-darwin-arm64.gz
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
	@rm -f vsp-client/device-client*

# ==================== 开发 ====================

dev-server:
	@cd vsp-server && go run ./cmd

dev-windows:
	@cd vsp-windows && wails dev

test:
	@cd tests/scripts && powershell -ExecutionPolicy Bypass -File ./full_integration_test.ps1

# ==================== 帮助 ====================

help:
	@echo "VSP Project Build System"
	@echo ""
	@echo "Targets:"
	@echo "  make build-server    - Build vsp-server (Linux)"
	@echo "  make build-client    - Build device-client (all platforms)"
	@echo "  make build-windows   - Build VSPManager (Windows GUI)"
	@echo "  make package         - Create release packages"
	@echo "  make release         - Show release instructions"
	@echo "  make clean           - Remove build artifacts"
	@echo "  make dev-server      - Run server in dev mode"
	@echo "  make dev-windows     - Run Windows client in dev mode"
	@echo "  make test            - Run integration tests"
	@echo ""
	@echo "Version: $(VERSION)"