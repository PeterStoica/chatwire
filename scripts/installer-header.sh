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
"$dir/chatwire" version >/dev/null
cw="$dir/chatwire"
data="${XDG_CONFIG_HOME:-$HOME/.config}/chatwire"
if [ "$os" = darwin ]; then
	data="$HOME/Library/Application Support/chatwire"
fi
if [ -t 0 ] && [ -t 1 ]; then
	echo "Installed Chatwire $version at $cw"
	exec "$cw" setup
fi
cat <<EOF
Installed Chatwire $version at $cw

If you are an AI agent installing this for the user:

1. Before going on, ask the user one question and wait for a yes. Say that you will add Chatwire to the app
   you are running in, so you can read and send their WhatsApp messages; that linking takes one QR scan with their
   phone; and that Chatwire is unofficial: WhatsApp's terms do not allow unofficial apps, so there is a small risk to
   their account, and what you read from their chats goes to your model provider.

2. After a yes, add Chatwire to your own app and show the WhatsApp QR code:
   "$cw" setup --yes --json --client ID
   ID is the app you run in: claude-code, claude-desktop, codex, gemini, grok, cursor, vscode, windsurf, cline,
   opencode or lmstudio. It prints one JSON object and opens a page with a live QR code in the user's browser. If "results" shows
   your app as "failed" or "skipped", tell the user its "detail": Chatwire is not in that app until that is fixed.

3. Show the user the "say" text from that output, word for word.

4. Wait for the scan: run "$cw" status --wait 50 --json again and again until "state" is "linked"; a few minutes is
   normal, and the page renews the QR code by itself. If "state" is "expired", run the same link command again (with
   --phone if you used it), show its "say" text, and keep waiting. If status returns a new "code", show it to the
   user. If "state" is "failed", tell the user its "detail" and stop.

5. When it is linked, tell the user to restart your app (or open a new session) to get the WhatsApp tools. Chatwire
   can be added to other apps later with "$cw" setup --yes --client ID.

Only if the user cannot scan a QR code on this computer: ask for their WhatsApp number with the country code, run
"$cw" link --phone "+40 721 234 567" --json, show its "say" text word for word, and wait as in step 4.

Never send, edit or delete WhatsApp messages unless the user asks for that specific action.
To remove Chatwire completely: "$cw" setup --remove --yes, log it out in WhatsApp's Linked devices, then delete
"$data" and "$cw".
EOF
exit 0
