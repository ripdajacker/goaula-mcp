#!/usr/bin/env bash
#
# Cross-compiles the aula-mcp binary for Linux, macOS and Windows.
# Output goes to dist/ as aula-mcp-<os>-<arch>[.exe], plus SHA256 checksums.
#
# Usage:
#   ./build.sh                # build all platforms
#   ./build.sh -v v1.2.3      # stamp a version string into the binary
#   TARGETS="linux/amd64 darwin/arm64" ./build.sh   # build a subset
set -euo pipefail

cd "$(dirname "$0")"

MODULE="github.com/ripdajacker/aula-api-client-go"
CMD="cmd/aula-mcp"
NAME="aula-mcp"
OUT_DIR="dist"
VERSION="${VERSION:-dev}"
COMMIT="$(git rev-parse --short HEAD 2>/dev/null || echo none)"
DATE="$(date -u +%Y-%m-%dT%H:%M:%SZ)"

while getopts "v:" opt; do
  case "$opt" in
    v) VERSION="$OPTARG" ;;
    *) echo "usage: $0 [-v version]" >&2; exit 1 ;;
  esac
done

LDFLAGS="-s -w -X main.version=${VERSION} -X main.commit=${COMMIT} -X main.date=${DATE}"

# os/arch[/arm] triples (env override must be captured before the array clobbers it)
OVERRIDE_TARGETS="${TARGETS:-}"
TARGETS=(
  linux/amd64
  linux/arm64
  linux/arm/7
  darwin/amd64
  darwin/arm64
  windows/amd64
  windows/arm64
)
if [[ -n "$OVERRIDE_TARGETS" ]]; then
  read -r -a TARGETS <<< "$OVERRIDE_TARGETS"
fi

mkdir -p "$OUT_DIR"

for target in "${TARGETS[@]}"; do
  IFS='/' read -r GOOS GOARCH GOARM <<< "$target"
  suffix=""
  [[ "$GOOS" == "windows" ]] && suffix=".exe"
  out="${OUT_DIR}/${NAME}-${GOOS}-${GOARCH}${suffix}"

  echo "==> building ${out}"
  CGO_ENABLED=0 GOOS="$GOOS" GOARCH="$GOARCH" GOARM="${GOARM:-}" \
    go build -trimpath -ldflags "$LDFLAGS" -o "$out" "${MODULE}/${CMD}"
done

echo "==> checksums"
if command -v shasum >/dev/null 2>&1; then
  (cd "$OUT_DIR" && shasum -a 256 ./"${NAME}"-* > checksums.txt)
else
  (cd "$OUT_DIR" && sha256sum ./"${NAME}"-* > checksums.txt)
fi

echo "==> done: $(ls "$OUT_DIR" | wc -l) files in ${OUT_DIR}/"
