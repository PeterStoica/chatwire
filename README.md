# Chatwire

Chatwire lets AI apps read, search and send your WhatsApp messages. It is an MCP server, so it works with Claude,
Codex, Gemini, Cursor, VS Code and other apps that support MCP.

Unofficial. Not affiliated with, endorsed or sponsored by WhatsApp or Meta. WhatsApp is a trademark of WhatsApp LLC.

## Install

The easy way: ask your AI app.

> Install Chatwire from github.com/PeterStoica/chatwire and link my WhatsApp.

It follows [AGENTS.md](AGENTS.md). You scan a QR code or type a code on your phone, restart the app, and you are done.

By hand, one line. It downloads the right build for your computer, checks it, and puts it in your user folder.

macOS and Linux:

```sh
curl -fsSL https://github.com/PeterStoica/chatwire/releases/latest/download/install.sh | sh
chatwire setup
```

Windows (PowerShell):

```powershell
irm https://github.com/PeterStoica/chatwire/releases/latest/download/install.ps1 | iex
chatwire setup
```

`chatwire setup` finds the AI apps on your computer, adds Chatwire to them, and walks you through linking WhatsApp.
`chatwire setup --remove` takes it out again. `chatwire update` installs a newer version when there is one;
`whatsapp_status` says when one is out.

## What your AI can do with it

- Tell you what you missed, with unread messages from every chat
- Read and search any chat, including history synced from your phone (linking asks for up to ten years), and page
  further back through the phone
- Send messages, replies, files, polls and forwards
- React, edit, delete, vote in polls and mark chats as read
- List your chats and groups, with members
- Rename groups, set descriptions, add or remove people, change admins, or leave a group
- Check whether numbers are on WhatsApp, with their about text
- Open photos, voice notes and documents

## How it works

Chatwire links to your WhatsApp as a companion device, the same way WhatsApp Web does. Your phone stays logged in.
It is one program with nothing else to install. Your messages are stored on your computer only, in a file only your
user can read. Chatwire talks to WhatsApp directly and to no one else.

Your AI can send files only from Desktop, Documents, Downloads, Pictures, Movies, Videos, Music and the temporary
folder, never hidden files, so a message crafted to trick it cannot send your keys or passwords. To allow more folders,
set `CHATWIRE_FILES` to a list of full paths (separated by `:`, or `;` on Windows) in the app's MCP settings.
To let an app only read, set `CHATWIRE_READ_ONLY=1` in its MCP settings: it then gets no tool that sends or changes
anything. Both settings apply to the app that sets them, even when several apps share Chatwire.

## Be aware

WhatsApp's Terms of Service do not allow unofficial clients. Using Chatwire could get your WhatsApp account
restricted or banned. Chatwire paces its sends to behave like a person, but the risk is yours.

Chatwire does not take calls. It answers call signals the way a computer without calling does, so your phone keeps
ringing as usual. If calls on your phone ever get stuck at "Connecting", unlink Chatwire in WhatsApp's Linked devices
and tell us.

## Build from source

With Go 1.27 or newer:

```sh
go install github.com/PeterStoica/chatwire/cmd/chatwire@latest
```

Or from a clone: `go build ./cmd/chatwire` and `go test ./...`.
