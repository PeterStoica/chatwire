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
version="${CHATWIRE_VERSION:-$(git describe --tags --always 2>/dev/null || echo dev)}"
header_lines=$(wc -l <scripts/installer-header.sh | tr -d ' ')
for target in darwin/arm64 darwin/amd64 linux/amd64 linux/arm64; do
	os="${target%/*}"
	arch="${target#*/}"
	binary="$out/chatwire_${os}_${arch}"
	if command -v sha256sum >/dev/null 2>&1; then
		sum=$(sha256sum "$binary" | cut -d' ' -f1)
	else
		sum=$(shasum -a 256 "$binary" | cut -d' ' -f1)
	fi
	installer="$out/chatwire_installer_${os}_${arch}.sh"
	sed -e "s|@PLATFORM@|$os/$arch|" -e "s|@VERSION@|$version|" -e "s|@SHA256@|$sum|" -e "s|@PAYLOAD_LINE@|$((header_lines + 1))|" scripts/installer-header.sh >"$installer"
	gzip -9 -c "$binary" | base64 | tr -d '\r\n' | fold -w 76 >>"$installer"
	echo >>"$installer"
	chmod 0755 "$installer"
done
cp scripts/install.sh scripts/install.ps1 LICENSE NOTICE "$out/"
cp internal/notices/notices.txt "$out/LICENSES.txt"
cd "$out"
if command -v sha256sum >/dev/null 2>&1; then
	sha256sum chatwire_*
else
	shasum -a 256 chatwire_*
fi | awk '{ name = $2; sub(/^\*/, "", name); print $1 "  " name }' > checksums.txt
