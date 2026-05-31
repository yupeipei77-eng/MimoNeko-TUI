#!/bin/bash
set -e

VERSION=${1:-"v0.1.1"}
OUTPUT_DIR="dist"

echo "Building MioNeko ${VERSION}..."

mkdir -p $OUTPUT_DIR

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
    
    output_name="mimoneko-${os}-${arch}"
    if [ "$os" = "windows" ]; then
        output_name="${output_name}.exe"
    fi
    
    echo "Building ${output_name}..."
    GOOS=$os GOARCH=$arch go build -o "${OUTPUT_DIR}/${output_name}" ./cmd/mimoneko
    
    # Package
    if [ "$os" = "windows" ]; then
        cd $OUTPUT_DIR && zip "mimoneko-${os}-${arch}.zip" "${output_name}" && rm "${output_name}" && cd ..
    else
        cd $OUTPUT_DIR && tar -czf "mimoneko-${os}-${arch}.tar.gz" "${output_name}" && rm "${output_name}" && cd ..
    fi
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
