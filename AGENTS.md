# Installing Chatwire for a user

You are an AI agent and the user asked you to install Chatwire, an MCP server that lets AI apps use their WhatsApp.
Every command is safe to run twice.

Before changing anything, ask the user one question and wait for a yes. Say that you will add Chatwire to the app you
are running in, so you can read and send their WhatsApp messages; that linking takes one QR scan with their phone; and
that Chatwire is unofficial: WhatsApp's terms do not allow unofficial apps, so there is a small risk to their account,
and what you read from their chats goes to your model provider. After a yes, do the rest yourself.

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

## 2. Add Chatwire to your own app and show the WhatsApp QR code

```sh
chatwire setup --yes --json --client ID
```

`ID` is the app you run in: `claude-code`, `claude-desktop`, `codex`, `gemini`, `cursor`, `vscode`, `windsurf`,
`cline`, `opencode` or `lmstudio`. Setup keeps everything else in that app's settings and, for apps it configures
through a file, backs up the file once, next to it, with the suffix `.before-chatwire`. It prints one JSON object and
opens a page with a live QR code in the user's browser. If `results` shows your app as `failed` or `skipped`, tell the
user its `detail`: Chatwire is not in that app until that is fixed. Chatwire can be added to other apps later, when the
user asks, with the same command.

## 3. Tell the user

Show the user the `say` text from that output, word for word, as one message. It has the page address, how to scan,
and a note about the risk of an unofficial app that the user must see before linking. If `state` is already `linked`,
tell them WhatsApp is linked and go to step 5.

## 4. Wait for the scan

```sh
chatwire status --wait 50 --json
```

Repeat until `state` is `linked`; a few minutes is normal. The page renews the QR code by itself while it is open. If
`state` is `expired`, run the same link command again (with `--phone` if you used it), show its `say` text, and keep
waiting. If status returns a new `code`, show it to the user. If `state` is `failed`, tell the user its `detail` and
stop.

## 5. Finish

Tell the user to restart your app (or open a new session) to get the WhatsApp tools.

## If the user cannot scan a QR code on this computer

For example on a remote or headless machine: ask for their WhatsApp mobile number with the country code, run
`chatwire link --phone "+40 721 234 567" --json`, show its `say` text word for word, and wait as in step 4.

## Rules

- Never send, edit or delete WhatsApp messages unless the user asks for that specific action.
- To remove Chatwire completely: `chatwire setup --remove --yes`, log it out in WhatsApp's Linked devices, then delete
  the `chatwire` data folder (`~/Library/Application Support/chatwire` on macOS, `~/.config/chatwire` on Linux,
  `%AppData%\chatwire` on Windows) and the program itself.
