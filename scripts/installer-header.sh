#!/bin/sh
set -eu
want="@PLATFORM@"
version="@VERSION@"
sum="@SHA256@"
payload=@PAYLOAD_LINE@
dir="${CHATWIRE_INSTALL_DIR:-$HOME/.local/bin}"
os=$(uname -s | tr '[:upper:]' '[:lower:]')
arch=$(uname -m)
case "$arch" in
x86_64 | amd64) arch=amd64 ;;
arm64 | aarch64) arch=arm64 ;;
esac
if [ "$os/$arch" != "$want" ]; then
	echo "chatwire: this installer is for $want, but this computer is $os/$arch" >&2
	exit 1
fi
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
if ! tail -n +"$payload" "$0" | base64 --decode 2>/dev/null | gunzip >"$tmp/chatwire" 2>/dev/null; then
	echo "chatwire: this installer is damaged; nothing was installed" >&2
	exit 1
fi
if command -v sha256sum >/dev/null 2>&1; then
	got=$(sha256sum "$tmp/chatwire" | cut -d' ' -f1)
else
	got=$(shasum -a 256 "$tmp/chatwire" | cut -d' ' -f1)
fi
if [ "$got" != "$sum" ]; then
	echo "chatwire: this installer is damaged; nothing was installed" >&2
	exit 1
fi
mkdir -p "$dir"
cp "$tmp/chatwire" "$dir/chatwire"
chmod 0755 "$dir/chatwire"
if command -v xattr >/dev/null 2>&1; then
	xattr -c "$dir/chatwire" 2>/dev/null || true
fi
"$dir/chatwire" version >/dev/null
cw="$dir/chatwire"
if [ -t 0 ] && [ -t 1 ]; then
	echo "Installed Chatwire $version at $cw"
	exec "$cw" setup
fi
cat <<EOF
Installed Chatwire $version at $cw

If you are an AI agent installing this for the user, finish the setup yourself:

1. Add Chatwire to the user's AI apps, then tell the user which apps were set up:
   "$cw" setup --yes --no-link --json

2. Tell the user that Chatwire is unofficial, that WhatsApp's terms do not allow unofficial apps so there is a
   small risk to their account, and that their messages stay only on this computer. Go on only if they agree.

3. Link WhatsApp. Skip this if "$cw" status --json already says "state": "linked". Otherwise ask the user
   whether they want a QR code or a code to type on the phone.
   - QR code: run "$cw" link --json. A page with the QR code opens in their browser. Tell them: on the phone,
     open WhatsApp, then Linked devices, then Link a device, and scan it.
   - Code: ask for their WhatsApp number with the country code, then run
     "$cw" link --phone "+40 721 234 567" --json. Show them the 8-character code exactly as returned and tell
     them: on the phone, open WhatsApp, then Linked devices, then Link a device, then "Link with phone number
     instead", and type it.
   Then run "$cw" status --wait 60 --json again and again until "state" is "linked". If it returns a
   different code, show the new one. If "state" is "expired", run the link command again.

4. Tell the user to restart the apps that were set up. The WhatsApp tools then appear in them.

Never send, edit or delete WhatsApp messages unless the user asks for that specific action.
To remove everything: "$cw" setup --remove --yes, then remove Chatwire in WhatsApp, Linked devices.
EOF
exit 0
