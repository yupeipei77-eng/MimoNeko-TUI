#!/bin/bash
set -e

# MimoNeko 安装脚本
# 用法: curl -fsSL https://raw.githubusercontent.com/yourusername/MimoNeko/main/install.sh | bash

REPO="mimoneko/mimoneko"
INSTALL_DIR="$HOME/.mimoneko/bin"
BINARY_NAME="mimoneko"

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# 打印带颜色的消息
info() { echo -e "${CYAN}[INFO]${NC} $1"; }
success() { echo -e "${GREEN}[OK]${NC} $1"; }
warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }
error() { echo -e "${RED}[ERROR]${NC} $1"; exit 1; }

# 检测操作系统
detect_os() {
    local os
    os="$(uname -s)"
    case "$os" in
        Linux*)     echo "linux";;
        Darwin*)    echo "mac";;
        MINGW*|MSYS*|CYGWIN*)  echo "windows";;
        *)          error "不支持的操作系统: $os";;
    esac
}

# 检测架构
detect_arch() {
    local arch
    arch="$(uname -m)"
    case "$arch" in
        x86_64|amd64)   echo "x86_64";;
        aarch64|arm64)   echo "arm64";;
        i386|i686)       echo "i386";;
        *)               error "不支持的架构: $arch";;
    esac
}

# 获取最新版本
get_latest_version() {
    local version
    version=$(curl -s "https://api.github.com/repos/$REPO/releases/latest" | grep '"tag_name"' | sed -E 's/.*"([^"]+)".*/\1/')
    if [ -z "$version" ]; then
        error "无法获取最新版本"
    fi
    echo "$version"
}

# 下载并安装
download_and_install() {
    local version="$1"
    local os="$2"
    local arch="$3"

    # 构建文件名
    local filename
    local ext="tar.gz"
    if [ "$os" = "windows" ]; then
        ext="zip"
    fi

    filename="${BINARY_NAME}-${os}-${arch}.${ext}"
    local url="https://github.com/$REPO/releases/download/${version}/${filename}"

    info "下载 $BINARY_NAME $version ($os/$arch)..."

    # 创建临时目录
    local tmpdir
    tmpdir=$(mktemp -d)
    trap "rm -rf $tmpdir" EXIT

    # 下载
    if ! curl -fsSL "$url" -o "$tmpdir/$filename"; then
        error "下载失败: $url"
    fi

    # 解压
    info "解压安装文件..."
    if [ "$ext" = "tar.gz" ]; then
        tar -xzf "$tmpdir/$filename" -C "$tmpdir"
    else
        unzip -q "$tmpdir/$filename" -d "$tmpdir"
    fi

    # 创建安装目录
    mkdir -p "$INSTALL_DIR"

    # 移动二进制文件
    local binary="$BINARY_NAME"
    if [ "$os" = "windows" ]; then
        binary="${BINARY_NAME}.exe"
    fi

    if [ -f "$tmpdir/$binary" ]; then
        mv "$tmpdir/$binary" "$INSTALL_DIR/$binary"
        chmod +x "$INSTALL_DIR/$binary"
        success "已安装到 $INSTALL_DIR/$binary"
    else
        error "未找到二进制文件"
    fi
}

# 添加到 PATH
add_to_path() {
    local shell_config=""
    local shell_type=""

    # 检测 shell 类型
    if [ -n "$BASH_VERSION" ]; then
        shell_type="bash"
    elif [ -n "$ZSH_VERSION" ]; then
        shell_type="zsh"
    elif [ -n "$FISH_VERSION" ]; then
        shell_type="fish"
    fi

    # 确定配置文件
    case "$shell_type" in
        fish)
            shell_config="$HOME/.config/fish/config.fish"
            ;;
        zsh)
            for f in "$HOME/.zshrc" "$HOME/.zshenv" "$HOME/.profile"; do
                if [ -f "$f" ]; then
                    shell_config="$f"
                    break
                fi
            done
            ;;
        bash|*)
            for f in "$HOME/.bashrc" "$HOME/.bash_profile" "$HOME/.profile"; do
                if [ -f "$f" ]; then
                    shell_config="$f"
                    break
                fi
            done
            ;;
    esac

    # 检查是否已在 PATH 中
    if echo "$PATH" | grep -q "$INSTALL_DIR"; then
        success "PATH 已包含安装目录"
        return
    fi

    # 添加到配置文件
    if [ -n "$shell_config" ] && [ -w "$shell_config" ]; then
        info "添加到 PATH ($shell_config)..."
        echo "" >> "$shell_config"
        echo "# MimoNeko" >> "$shell_config"

        if [ "$shell_type" = "fish" ]; then
            echo "fish_add_path $INSTALL_DIR" >> "$shell_config"
        else
            echo "export PATH=\"$INSTALL_DIR:\$PATH\"" >> "$shell_config"
        fi

        success "已添加到 PATH"
        warn "请运行 'source $shell_config' 或重新打开终端"
    else
        warn "无法自动添加到 PATH"
        echo ""
        echo "请手动添加以下内容到你的 shell 配置文件:"
        if [ "$shell_type" = "fish" ]; then
            echo "  fish_add_path $INSTALL_DIR"
        else
            echo "  export PATH=\"$INSTALL_DIR:\$PATH\""
        fi
    fi

    # GitHub Actions 支持
    if [ -n "$GITHUB_PATH" ]; then
        echo "$INSTALL_DIR" >> "$GITHUB_PATH"
    fi
}

# 检查已安装版本
check_existing() {
    if command -v "$BINARY_NAME" &>/dev/null; then
        local current_version
        current_version=$("$BINARY_NAME" --version 2>/dev/null | head -1 || echo "unknown")
        info "已安装版本: $current_version"
        return 0
    fi
    return 1
}

# 主函数
main() {
    echo ""
    echo -e "${CYAN}=====================================${NC}"
    echo -e "${CYAN}  MimoNeko 安装程序${NC}"
    echo -e "${CYAN}=====================================${NC}"
    echo ""

    # 检测环境
    local os arch
    os=$(detect_os)
    arch=$(detect_arch)
    info "检测到系统: $os/$arch"

    # 检查已安装版本
    check_existing && true

    # 获取版本
    local version="${VERSION:-$(get_latest_version)}"
    info "安装版本: $version"

    # 下载安装
    download_and_install "$version" "$os" "$arch"

    # 配置 PATH
    add_to_path

    echo ""
    echo -e "${GREEN}=====================================${NC}"
    echo -e "${GREEN}  安装完成！${NC}"
    echo -e "${GREEN}=====================================${NC}"
    echo ""
    echo "运行以下命令开始使用:"
    echo "  $BINARY_NAME --help"
    echo ""
    echo "配置 API Key:"
    echo "  export MIMO_API_KEY=\"your-api-key\""
    echo ""
}

main "$@"
