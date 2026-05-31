# MimoNeko Makefile
# 用法: make install

# 变量
BINARY_NAME=mimoneko
INSTALL_DIR=$(HOME)/.mimoneko/bin
VERSION?=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")

# Go 参数
GO=go
GOFLAGS=-ldflags "-s -w -X main.version=$(VERSION)"
GOROOT=$(shell $(GO) env GOROOT)

# 平台检测
OS=$(shell uname -s | tr A-Z a-z)
ifeq ($(OS),darwin)
    OS=mac
endif
ifeq ($(findstring MINGW,$(shell uname -s)),MINGW)
    OS=windows
endif
ifeq ($(findstring MSYS,$(shell uname -s)),MSYS)
    OS=windows
endif

ARCH=$(shell uname -m)
ifeq ($(ARCH),aarch64)
    ARCH=arm64
endif

# Windows 特殊处理
ifeq ($(OS),windows)
    BINARY_NAME=mimoneko.exe
    INSTALL_DIR=$(USERPROFILE)/.mimoneko/bin
    EXT=.exe
endif

.PHONY: all build install uninstall clean test lint help

## 默认目标
all: build

## 构建
build:
	@echo "构建 $(BINARY_NAME)..."
	$(GO) build $(GOFLAGS) -o $(BINARY_NAME) ./cmd/neko
	@echo "构建完成: $(BINARY_NAME)"

## 安装到本地
install: build
	@echo "安装 $(BINARY_NAME) 到 $(INSTALL_DIR)..."
	@mkdir -p $(INSTALL_DIR)
	@cp $(BINARY_NAME) $(INSTALL_DIR)/$(BINARY_NAME)
	@chmod +x $(INSTALL_DIR)/$(BINARY_NAME)
	@echo ""
	@echo "已安装到: $(INSTALL_DIR)/$(BINARY_NAME)"
	@echo ""
	@if echo "$$PATH" | grep -q "$(INSTALL_DIR)"; then \
		echo "✓ PATH 已包含安装目录"; \
	else \
		echo "⚠ 请将以下内容添加到你的 shell 配置文件:"; \
		echo ""; \
		if [ "$(OS)" = "windows" ]; then \
			echo "  set PATH=%PATH%;$(INSTALL_DIR)"; \
		else \
			echo "  export PATH=\"$(INSTALL_DIR):\$$PATH\""; \
		fi; \
		echo ""; \
	fi

## 卸载
uninstall:
	@echo "卸载 $(BINARY_NAME)..."
	@rm -f $(INSTALL_DIR)/$(BINARY_NAME)
	@echo "已卸载"

## 清理构建文件
clean:
	@echo "清理构建文件..."
	@rm -f $(BINARY_NAME)
	@rm -rf dist/
	@echo "清理完成"

## 运行测试
test:
	@echo "运行测试..."
	$(GO) test ./... -v

## 运行 lint
lint:
	@echo "运行 linter..."
	golangci-lint run ./...

## 构建所有平台版本
build-all:
	@echo "构建所有平台版本..."
	@mkdir -p dist
	GOOS=linux GOARCH=amd64 $(GO) build $(GOFLAGS) -o dist/$(BINARY_NAME)-linux-x86_64 ./cmd/neko
	GOOS=linux GOARCH=arm64 $(GO) build $(GOFLAGS) -o dist/$(BINARY_NAME)-linux-arm64 ./cmd/neko
	GOOS=darwin GOARCH=amd64 $(GO) build $(GOFLAGS) -o dist/$(BINARY_NAME)-mac-x86_64 ./cmd/neko
	GOOS=darwin GOARCH=arm64 $(GO) build $(GOFLAGS) -o dist/$(BINARY_NAME)-mac-arm64 ./cmd/neko
	GOOS=windows GOARCH=amd64 $(GO) build $(GOFLAGS) -o dist/$(BINARY_NAME)-windows-x86_64.exe ./cmd/neko
	@echo "构建完成，文件在 dist/ 目录"

## 创建发布包
release: build-all
	@echo "创建发布包..."
	@cd dist && tar -czf $(BINARY_NAME)-linux-x86_64.tar.gz $(BINARY_NAME)-linux-x86_64
	@cd dist && tar -czf $(BINARY_NAME)-linux-arm64.tar.gz $(BINARY_NAME)-linux-arm64
	@cd dist && tar -czf $(BINARY_NAME)-mac-x86_64.tar.gz $(BINARY_NAME)-mac-x86_64
	@cd dist && tar -czf $(BINARY_NAME)-mac-arm64.tar.gz $(BINARY_NAME)-mac-arm64
	@cd dist && zip -q $(BINARY_NAME)-windows-x86_64.zip $(BINARY_NAME)-windows-x86_64.exe
	@echo "发布包创建完成"

## 显示帮助
help:
	@echo "MimoNeko Makefile"
	@echo ""
	@echo "用法:"
	@echo "  make [target]"
	@echo ""
	@echo "目标:"
	@echo "  all          构建项目 (默认)"
	@echo "  build        构建二进制文件"
	@echo "  install      构建并安装到 ~/.mimoneko/bin"
	@echo "  uninstall    卸载"
	@echo "  clean        清理构建文件"
	@echo "  test         运行测试"
	@echo "  lint         运行 linter"
	@echo "  build-all    构建所有平台版本"
	@echo "  release      创建发布包"
	@echo "  help         显示此帮助"
