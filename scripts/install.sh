#!/bin/sh
set -eu
repo="PeterStoica/chatwire"
base="${CHATWIRE_BASE_URL:-https://github.com/$repo/releases/latest/download}"
dir="${CHATWIRE_INSTALL_DIR:-$HOME/.local/bin}"
os=$(uname -s | tr '[:upper:]' '[:lower:]')
case "$os" in
darwin | linux) ;;
*)
	echo "chatwire: this installer is for macOS and Linux; on Windows use install.ps1" >&2
	exit 1
	;;
esac
arch=$(uname -m)
case "$arch" in
x86_64 | amd64) arch=amd64 ;;
arm64 | aarch64) arch=arm64 ;;
*)
	echo "chatwire: no build for this processor ($arch)" >&2
	exit 1
	;;
esac
asset="chatwire_${os}_${arch}"
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
fetch() {
	if command -v curl >/dev/null 2>&1; then
		curl -fsSL "$1" -o "$2"
	else
		wget -qO "$2" "$1"
	fi
}
fetch "$base/$asset" "$tmp/$asset"
fetch "$base/checksums.txt" "$tmp/checksums.txt"
want=$(awk -v name="$asset" '$2 == name { print $1 }' "$tmp/checksums.txt")
if command -v sha256sum >/dev/null 2>&1; then
	got=$(sha256sum "$tmp/$asset" | cut -d' ' -f1)
else
	got=$(shasum -a 256 "$tmp/$asset" | cut -d' ' -f1)
fi
if [ -z "$want" ] || [ "$want" != "$got" ]; then
	echo "chatwire: the download does not match its checksum; nothing was installed" >&2
	exit 1
fi
mkdir -p "$dir"
cp "$tmp/$asset" "$dir/chatwire"
chmod 0755 "$dir/chatwire"
echo "Installed $dir/chatwire ($("$dir/chatwire" version))"
case ":$PATH:" in
*":$dir:"*) echo "Next: chatwire setup" ;;
*) echo "Next: $dir/chatwire setup   ($dir is not on your PATH yet)" ;;
esac
