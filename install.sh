#!/bin/bash
#
# klastr Installer Script
# Usage: curl -fsSL https://raw.githubusercontent.com/alessandropitocchi/deploy-cluster/main/install.sh | bash
#

set -e

# Configuration
REPO="alessandropitocchi/deploy-cluster"
BINARY_NAME="klastr"
INSTALL_DIR="/usr/local/bin"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Print functions
print_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

print_success() {
    echo -e "${GREEN}[OK]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

# Detect OS and architecture
detect_platform() {
    local os=$(uname -s | tr '[:upper:]' '[:lower:]')
    local arch=$(uname -m)

    case "$arch" in
        x86_64|amd64)
            arch="amd64"
            ;;
        arm64|aarch64)
            arch="arm64"
            ;;
        *)
            print_error "Unsupported architecture: $arch"
            exit 1
            ;;
    esac

    case "$os" in
        linux|darwin)
            ;;
        *)
            print_error "Unsupported OS: $os"
            exit 1
            ;;
    esac

    echo "${os}_${arch}"
}

# Get latest release version
get_latest_version() {
    local version
    version=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')
    if [ -z "$version" ]; then
        print_error "Could not determine latest version"
        exit 1
    fi
    echo "$version"
}

# Download binary
download_binary() {
    local version=$1
    local platform=$2
    local tmp_dir=$3
    
    local download_url="https://github.com/${REPO}/releases/download/${version}/${BINARY_NAME}_${version#v}_${platform}.tar.gz"
    local checksum_url="https://github.com/${REPO}/releases/download/${version}/checksums.txt"
    
    print_info "Downloading ${BINARY_NAME} ${version} for ${platform}..."
    
    # Download archive
    if ! curl -fsSL "$download_url" -o "${tmp_dir}/${BINARY_NAME}.tar.gz"; then
        print_error "Failed to download binary"
        print_info "URL: $download_url"
        exit 1
    fi
    
    # Extract
    tar -xzf "${tmp_dir}/${BINARY_NAME}.tar.gz" -C "$tmp_dir"
    
    print_success "Download complete"
}

# Install binary
install_binary() {
    local tmp_dir=$1
    local binary_path="${tmp_dir}/${BINARY_NAME}"
    
    # Check if we need sudo
    local sudo_cmd=""
    if [ ! -w "$INSTALL_DIR" ]; then
        sudo_cmd="sudo"
        print_info "Installing to ${INSTALL_DIR} (requires sudo)..."
    fi
    
    # Move binary to install directory
    $sudo_cmd mv "$binary_path" "${INSTALL_DIR}/${BINARY_NAME}"
    $sudo_cmd chmod +x "${INSTALL_DIR}/${BINARY_NAME}"
    
    print_success "Installed to ${INSTALL_DIR}/${BINARY_NAME}"
}

# Check if binary is in PATH
check_path() {
    if ! command -v "$BINARY_NAME" &> /dev/null; then
        print_warning "${INSTALL_DIR} is not in your PATH"
        print_info "Add the following to your shell profile:"
        echo "  export PATH=\"\$PATH:${INSTALL_DIR}\""
        return 1
    fi
    return 0
}

# Verify installation
verify_installation() {
    print_info "Verifying installation..."
    
    if command -v "$BINARY_NAME" &> /dev/null; then
        local version
        version=$($BINARY_NAME --version 2>/dev/null || echo "unknown")
        print_success "Installation verified!"
        print_info "Version: $version"
    else
        print_error "Installation verification failed"
        exit 1
    fi
}

# Check dependencies
check_dependencies() {
    print_info "Checking dependencies..."
    
    local deps=("curl" "tar")
    for dep in "${deps[@]}"; do
        if ! command -v "$dep" &> /dev/null; then
            print_error "Required dependency not found: $dep"
            exit 1
        fi
    done
    
    print_success "All dependencies satisfied"
}

# Print usage
print_usage() {
    echo ""
    echo "Usage:"
    echo "  $BINARY_NAME [command]"
    echo ""
    echo "Quick Start:"
    echo "  $BINARY_NAME init              # Generate starter template"
    echo "  $BINARY_NAME run               # Deploy cluster"
    echo "  $BINARY_NAME status            # Check cluster status"
    echo "  $BINARY_NAME env list          # List environments"
    echo ""
    echo "Documentation: https://github.com/${REPO}"
}

# Main installation flow
main() {
    echo ""
    echo "╔════════════════════════════════════════╗"
    echo "║         klastr Installer               ║"
    echo "║  Kubernetes Cluster Deployment Tool    ║"
    echo "╚════════════════════════════════════════╝"
    echo ""

    # Check dependencies
    check_dependencies
    
    # Detect platform
    local platform
    platform=$(detect_platform)
    print_info "Detected platform: $platform"
    
    # Get version
    local version
    version=$(get_latest_version)
    print_info "Latest version: $version"
    
    # Create temp directory
    local tmp_dir
    tmp_dir=$(mktemp -d)
    trap "rm -rf $tmp_dir" EXIT
    
    # Download and install
    download_binary "$version" "$platform" "$tmp_dir"
    install_binary "$tmp_dir"
    
    # Verify
    verify_installation
    
    # Check PATH
    if check_path; then
        print_success "Installation complete!"
        print_usage
    else
        print_warning "Installation complete, but PATH needs to be updated"
        print_usage
    fi
}

# Allow version override for testing
if [ -n "$KLASTR_VERSION" ]; then
    VERSION="$KLASTR_VERSION"
fi

# Run main
main
