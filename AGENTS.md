# Installing Chatwire for a user

You are an AI agent and the user asked you to install Chatwire, an MCP server that lets AI apps use their WhatsApp.
Finish it yourself without asking the user anything. Every command is safe to run twice.

## 1. Install the binary

No Go or other tools are needed; the release has a ready build for every system.

macOS or Linux:

```sh
curl -fsSL https://github.com/PeterStoica/chatwire/releases/latest/download/install.sh | sh
```

Windows (PowerShell):

```powershell
irm https://github.com/PeterStoica/chatwire/releases/latest/download/install.ps1 | iex
```

The installer checks the download against the release checksums and prints where it put `chatwire`
(`~/.local/bin` on macOS and Linux, `%LOCALAPPDATA%\Programs\chatwire` on Windows). If that folder is not on the PATH,
use the full path it printed in the steps below. Developers with Go 1.27 can instead run
`go install github.com/PeterStoica/chatwire/cmd/chatwire@latest`.

## 2. Add Chatwire to the user's AI apps and show the WhatsApp QR code

```sh
chatwire setup --yes --json
```

This adds Chatwire to every AI app it finds (Claude Code, Claude Desktop, Codex, Gemini CLI, Cursor, VS Code, Windsurf,
Cline, opencode, LM Studio), keeping everything else in their settings and backing up each file it changes once, next
to it, with the suffix `.before-chatwire`. It then opens a page with a live QR code in the user's browser. To limit it
to some apps: `--client claude-desktop,codex`.

## 3. Tell the user

Show the user the `say` text from that output, word for word, as one message. It has the page address, how to scan,
and a note about the risk of an unofficial app that the user must see before linking. If `state` is already `linked`,
tell them WhatsApp is linked and go to step 5.

## 4. Wait for the scan

```sh
chatwire status --wait 50 --json
```

Repeat until `state` is `linked`; a few minutes is normal. The page renews the QR code by itself while it is open. If
`state` is `expired`, run `chatwire link --json`, show its `say` text, and keep waiting.

## 5. Finish

Tell the user to restart the apps that were set up (or open a new session) to get the WhatsApp tools.

## If the user cannot scan a QR code on this computer

For example on a remote or headless machine: ask for their WhatsApp mobile number with the country code, run
`chatwire link --phone "+40 721 234 567" --json`, show its `say` text word for word, and wait as in step 4.

## Rules

- Never send, edit or delete WhatsApp messages unless the user asks for that specific action.
- To undo everything: `chatwire setup --remove --yes`, then remove Chatwire in WhatsApp, Linked devices.
