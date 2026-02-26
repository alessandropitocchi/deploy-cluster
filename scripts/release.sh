#!/bin/bash
#
# Build release binaries for klastr
# Creates binaries for linux/darwin amd64/arm64
#

set -e

VERSION=${1:-$(git describe --tags --always --dirty)}
REPO="github.com/alessandropitocchi/deploy-cluster"
BINARY_NAME="klastr"

# Colors
GREEN='\033[0;32m'
BLUE='\033[0;34m'
NC='\033[0m'

echo -e "${BLUE}Building klastr ${VERSION}${NC}"
echo ""

# Create dist directory
mkdir -p dist

# Build matrix
PLATFORMS=(
    "linux/amd64"
    "linux/arm64"
    "darwin/amd64"
    "darwin/arm64"
)

for platform in "${PLATFORMS[@]}"; do
    GOOS=${platform%/*}
    GOARCH=${platform#*/}
    
    output_name="${BINARY_NAME}_${VERSION#v}_${GOOS}_${GOARCH}"
    
    echo -e "${BLUE}Building for ${GOOS}/${GOARCH}...${NC}"
    
    GOOS=$GOOS GOARCH=$GOARCH go build \
        -ldflags "-s -w -X main.Version=${VERSION}" \
        -o "dist/${BINARY_NAME}" \
        ./cmd/deploycluster
    
    # Create tar.gz
    tar -czf "dist/${output_name}.tar.gz" -C dist "${BINARY_NAME}"
    
    # Calculate checksum
    shasum -a 256 "dist/${output_name}.tar.gz" | awk '{print $1}' > "dist/${output_name}.tar.gz.sha256"
    
    # Remove binary (keep only archive)
    rm "dist/${BINARY_NAME}"
    
    echo -e "${GREEN}✓${NC} dist/${output_name}.tar.gz"
done

# Generate checksums file
cd dist
shasum -a 256 *.tar.gz > checksums.txt
cd ..

echo ""
echo -e "${GREEN}Build complete!${NC}"
echo ""
echo "Artifacts in dist/:"
ls -lh dist/
