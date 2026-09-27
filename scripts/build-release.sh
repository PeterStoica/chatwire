#!/bin/sh
set -eu
out="${1:-dist}"
rm -rf "$out"
mkdir -p "$out"
for target in darwin/arm64 darwin/amd64 linux/amd64 linux/arm64 windows/amd64 windows/arm64; do
	os="${target%/*}"
	arch="${target#*/}"
	ext=""
	if [ "$os" = windows ]; then
		ext=".exe"
	fi
	CGO_ENABLED=0 GOOS="$os" GOARCH="$arch" go build -trimpath -ldflags "-s -w" -o "$out/chatwire_${os}_${arch}${ext}" ./cmd/chatwire
done
cp scripts/install.sh scripts/install.ps1 "$out/"
cd "$out"
if command -v sha256sum >/dev/null 2>&1; then
	sha256sum chatwire_* > checksums.txt
else
	shasum -a 256 chatwire_* > checksums.txt
fi
