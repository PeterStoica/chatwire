# Installing Chatwire for a user

You are an AI agent and the user asked you to install Chatwire, an MCP server that lets AI apps use their WhatsApp.
Follow these steps in order. Every command is safe to run twice.

## 1. Install the binary

If Go 1.27 or newer is installed (`go version`):

```sh
go install github.com/PeterStoica/chatwire/cmd/chatwire@latest
```

The binary lands in `$(go env GOPATH)/bin`. Use that full path below if it is not on the PATH.

## 2. Register it with the user's AI apps

```sh
chatwire setup --yes --no-link --json
```

This adds Chatwire to every AI app it finds (Claude Code, Claude Desktop, Codex, Gemini CLI, Cursor, VS Code, Windsurf,
Cline, opencode, LM Studio). It keeps everything else in their settings, and backs up each file it changes once, next to
it, with the suffix `.before-chatwire`. To limit it to some apps: `--client claude-desktop,codex`.
Tell the user which apps were set up.

## 3. Link WhatsApp

Check first:

```sh
chatwire status --json
```

If `state` is `linked`, skip to step 4. Otherwise ask the user how they want to link:

- **QR code** (they are at this computer): run `chatwire link --json`. A page with the QR code opens in their browser.
  Tell them: on the phone, open WhatsApp, then Linked devices, then Link a device, and scan the code on the page.
- **Code** (any situation): ask for their WhatsApp mobile number with the country code, then run
  `chatwire link --phone "+40 721 234 567" --json`. Show them the 8-character `code` exactly as returned, and tell them:
  on the phone, open WhatsApp, then Linked devices, then Link a device, then "Link with phone number instead", and type it.

Then wait for them:

```sh
chatwire status --wait 60 --json
```

Repeat until `state` is `linked`. If a code or QR expires, run the link command again.

## 4. Finish

Tell the user to restart the apps that were set up (or open a new session). The WhatsApp tools then appear in them.

## Rules

- Never send, edit or delete WhatsApp messages unless the user asks for that specific action.
- Linking gives this computer access to the user's messages. Make sure the user wants that before step 3.
- To undo everything: `chatwire setup --remove --yes`.
