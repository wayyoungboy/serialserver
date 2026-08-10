# VSP CLI Makefile

# 设置
VERSION = 1.0.0
GO = go
GOFLAGS = -v

# 目标平台
GOOS ?= $(shell $(GO) env GOOS)
GOARCH ?= $(shell $(GO) env GOARCH)

# 源文件
PKGS = $(shell $(GO) list ./...)

DIST = dist
DEVICE_AGENT = device-agent
DESKTOP_GATEWAY = desktop-gateway

# 颜色
RED = \033[0;31m
GREEN = \033[0;32m
YELLOW = \033[0;33m
NC = \033[0m

.PHONY: all build build-windows build-linux build-darwin clean test deps help

# 默认目标
all: build

# 帮助信息
help:
	@echo "VSP CLI clients v$(VERSION)"
	@echo ""
	@echo "用法:"
	@echo "  make build          # 构建当前平台的 device-agent 和 desktop-gateway"
	@echo "  make build-windows # 构建 Windows 版本"
	@echo "  make build-linux   # 构建 Linux 版本"
	@echo "  make build-darwin  # 构建 macOS 版本"
	@echo "  make clean         # 清理构建产物"
	@echo "  make test          # 运行测试"
	@echo "  make deps          # 下载依赖"
	@echo ""

# 下载依赖
deps:
	@echo "下载依赖..."
	$(GO) mod download

# 构建当前平台
build:
	@echo "构建 $(GOOS)/$(GOARCH) 版本..."
	$(GO) build $(GOFLAGS) -o $(DEVICE_AGENT) ./cmd/device-agent
	$(GO) build $(GOFLAGS) -o $(DESKTOP_GATEWAY) ./cmd/desktop-gateway
	@echo "构建完成: $(DEVICE_AGENT), $(DESKTOP_GATEWAY)"

# 构建Windows版本
build-windows:
	@echo "构建 Windows 版本..."
	@mkdir -p $(DIST)
	GOOS=windows GOARCH=amd64 $(GO) build $(GOFLAGS) -o $(DIST)/$(DEVICE_AGENT)-windows-amd64.exe ./cmd/device-agent
	GOOS=windows GOARCH=amd64 $(GO) build $(GOFLAGS) -o $(DIST)/$(DESKTOP_GATEWAY)-windows-amd64.exe ./cmd/desktop-gateway
	@echo "构建完成: $(DIST)"

# 构建Linux版本
build-linux:
	@echo "构建 Linux 版本..."
	@mkdir -p $(DIST)
	GOOS=linux GOARCH=amd64 $(GO) build $(GOFLAGS) -o $(DIST)/$(DEVICE_AGENT)-linux-amd64 ./cmd/device-agent
	GOOS=linux GOARCH=amd64 $(GO) build $(GOFLAGS) -o $(DIST)/$(DESKTOP_GATEWAY)-linux-amd64 ./cmd/desktop-gateway
	@echo "构建完成: $(DIST)"

# 构建macOS版本
build-darwin:
	@echo "构建 macOS 版本..."
	@if [ "$$(uname -s)" != "Darwin" ]; then echo "macOS artifacts must be built on macOS because the serial dependency uses Darwin build tags."; exit 1; fi
	@mkdir -p $(DIST)
	GOOS=darwin GOARCH=$(GOARCH) CGO_ENABLED=1 $(GO) build $(GOFLAGS) -o $(DIST)/$(DEVICE_AGENT)-darwin-$(GOARCH) ./cmd/device-agent
	GOOS=darwin GOARCH=$(GOARCH) CGO_ENABLED=1 $(GO) build $(GOFLAGS) -o $(DIST)/$(DESKTOP_GATEWAY)-darwin-$(GOARCH) ./cmd/desktop-gateway
	@echo "构建完成: $(DIST)"

# 构建所有平台
build-all: build-windows build-linux
	@echo "所有平台构建完成!"
	@echo "macOS artifacts are built by the GitHub Actions macOS runner or by running make build-darwin on macOS."

# 清理
clean:
	@echo "清理构建产物..."
	rm -f $(DEVICE_AGENT) $(DESKTOP_GATEWAY)
	rm -rf $(DIST)
	@echo "清理完成"

# 测试
test:
	@echo "运行测试..."
	$(GO) test -v $(PKGS)

# 运行
run: build
	@echo "运行 device-agent --help..."
	./$(DEVICE_AGENT) -h

# 安装
install: build
	@echo "安装程序..."
	cp $(DEVICE_AGENT) /usr/local/bin/
	cp $(DESKTOP_GATEWAY) /usr/local/bin/
	@echo "安装完成"

# 代码格式检查
fmt:
	$(GO) fmt ./...

# 代码审查
lint:
	@echo "代码审查..."
	$(GO) vet ./...

# 发布准备
release: clean build-all
	@echo "发布版本 v$(VERSION) 准备完成!"
