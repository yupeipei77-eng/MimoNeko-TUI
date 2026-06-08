#!/bin/bash
set -e

VERSION=${1:-"v0.1.4-beta"}
VERSION_NAME=${VERSION#v}
OUTPUT_DIR="dist"
LDFLAGS="-X github.com/yupeipei77-eng/MimoNeko-TUI/internal/version.Version=${VERSION_NAME}"

echo "Building MimoNeko ${VERSION}..."

rm -rf "$OUTPUT_DIR"
mkdir -p "$OUTPUT_DIR"

# Build matrix
platforms=(
    "windows/amd64"
    "windows/arm64"
    "linux/amd64"
    "linux/arm64"
    "darwin/arm64"
)

for platform in "${platforms[@]}"; do
    IFS='/' read -r os arch <<< "$platform"

    package_name="mimoneko-${os}-${arch}"
    package_dir="${OUTPUT_DIR}/pkg-${os}-${arch}"
    mkdir -p "$package_dir"

    binary_name="mimoneko"
    alias_name="neko"
    if [ "$os" = "windows" ]; then
        binary_name="mimoneko.exe"
        alias_name="neko.exe"
    fi

    echo "Building ${package_name}..."
    GOOS=$os GOARCH=$arch go build -ldflags "$LDFLAGS" -o "${package_dir}/${binary_name}" ./cmd/mimoneko
    GOOS=$os GOARCH=$arch go build -ldflags "$LDFLAGS" -o "${package_dir}/${alias_name}" ./cmd/neko

    if [ "$os" = "windows" ]; then
        cp install.ps1 start-mimoneko.bat "$package_dir/"
        (cd "$package_dir" && zip -q "../${package_name}.zip" ./*)
    else
        cp install.sh "$package_dir/"
        chmod +x "$package_dir/install.sh" "$package_dir/mimoneko" "$package_dir/neko"
        tar -czf "${OUTPUT_DIR}/${package_name}.tar.gz" -C "$package_dir" .
    fi
    rm -rf "$package_dir"
done

# Generate SHA256
echo "Generating SHA256 checksums..."
cd $OUTPUT_DIR && sha256sum * > SHA256SUMS && cd ..

echo ""
echo "Build complete! Files in ${OUTPUT_DIR}/"
echo ""
ls -la $OUTPUT_DIR/
echo ""
echo "SHA256SUMS:"
cat $OUTPUT_DIR/SHA256SUMS
