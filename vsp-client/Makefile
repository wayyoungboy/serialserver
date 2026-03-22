# VSP Manager Makefile

# 设置
NAME = vsp-manager
VERSION = 1.0.0
AUTHOR = 韦永博
GO = go
GOFLAGS = -v

# 目标平台
GOOS ?= $(shell $(GO) env GOOS)
GOARCH ?= $(shell $(GO) env GOARCH)

# 源文件
PKGS = $(shell $(GO) list ./...)

# 输出
DIST = dist
WIN_EXE = $(NAME).exe
LIN_BIN = $(NAME)
MAC_BIN = $(NAME)_darwin_amd64

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
	@echo "VSP Manager - 虚拟串口管理器 v$(VERSION)"
	@echo ""
	@echo "用法:"
	@echo "  make build          # 构建当前平台版本"
	@echo "  make build-windows # 构建Windows版本"
	@echo "  make build-linux   # 构建Linux版本"
	@echo "  make build-darwin  # 构建macOS版本"
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
	$(GO) build $(GOFLAGS) -o $(NAME) ./cmd
	@echo "构建完成: $(NAME)"

# 构建Windows版本
build-windows:
	@echo "构建 Windows 版本..."
	GOOS=windows GOARCH=amd64 $(GO) build $(GOFLAGS) -o $(WIN_EXE) ./cmd
	@echo "构建完成: $(WIN_EXE)"

# 构建Linux版本
build-linux:
	@echo "构建 Linux 版本..."
	GOOS=linux GOARCH=amd64 $(GO) build $(GOFLAGS) -o $(LIN_BIN) ./cmd
	@echo "构建完成: $(LIN_BIN)"

# 构建macOS版本
build-darwin:
	@echo "构建 macOS 版本..."
	GOOS=darwin GOARCH=amd64 $(GO) build $(GOFLAGS) -o $(MAC_BIN) ./cmd
	@echo "构建完成: $(MAC_BIN)"

# 构建所有平台
build-all: build-windows build-linux build-darwin
	@echo "所有平台构建完成!"
	@echo "  - $(WIN_EXE)"
	@echo "  - $(LIN_BIN)"
	@echo "  - $(MAC_BIN)"

# 清理
clean:
	@echo "清理构建产物..."
	rm -f $(NAME) $(WIN_EXE) $(LIN_BIN) $(MAC_BIN)
	rm -rf $(DIST)
	@echo "清理完成"

# 测试
test:
	@echo "运行测试..."
	$(GO) test -v $(PKGS)

# 运行
run: build
	@echo "运行程序..."
	./$(NAME) -version

# 安装
install: build
	@echo "安装程序..."
	cp $(NAME) /usr/local/bin/
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
