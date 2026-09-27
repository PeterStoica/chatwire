# Chatwire

Chatwire lets AI apps read, search and send your WhatsApp messages. It is an MCP server, so it works with Claude,
Codex, Gemini, Cursor, VS Code and other apps that support MCP.

Unofficial. Not affiliated with, endorsed or sponsored by WhatsApp or Meta. WhatsApp is a trademark of WhatsApp LLC.

## Install

The easy way: ask your AI app.

> Install Chatwire from github.com/PeterStoica/chatwire and link my WhatsApp.

It follows [AGENTS.md](AGENTS.md): it asks you once, adds Chatwire to itself, and opens a page with a QR code. You scan
it with your phone, restart the app, and you are done. You can also give your AI app the installer file for your
computer from the [latest release](https://github.com/PeterStoica/chatwire/releases/latest), for example
`chatwire_installer_darwin_arm64.sh` for a Mac with Apple silicon; the file itself tells the AI what to do.

By hand, one line. It downloads the right build for your computer, checks it against the release checksums, and
puts it in your user folder.

macOS and Linux:

```sh
curl -fsSL https://github.com/PeterStoica/chatwire/releases/latest/download/install.sh | sh
chatwire setup
```

If `~/.local/bin` is not on your PATH, run the full path the installer prints instead of `chatwire`.

Windows (PowerShell):

```powershell
irm https://github.com/PeterStoica/chatwire/releases/latest/download/install.ps1 | iex
chatwire setup
```

Open a new terminal before `chatwire setup`, so it finds the program.

`chatwire setup` lists the AI apps it finds, asks before changing them, then shows a page with a QR code to link
WhatsApp. `chatwire setup --client claude-code` adds it to one app only, and `chatwire setup --remove` takes it out
again. `chatwire help` shows every command.

## What your AI can do with it

- Tell you what you missed, with your unread messages
- Read and search any chat, including history synced from your phone (linking asks for up to ten years), and page
  further back through the phone
- Send messages, replies, files, polls and forwards
- React, edit, delete, vote in polls and mark chats as read
- List your chats and groups, with members
- Rename groups, set descriptions, add or remove people, change admins, or leave a group
- Check whether numbers are on WhatsApp, with their about text
- Open photos, videos, voice notes and documents

## How it works

Chatwire links to your WhatsApp as a companion device, the same way WhatsApp Web does. Your phone stays logged in.
It is one program with nothing else to install. It keeps your messages on your computer, in your own user folder
where other users of the computer cannot read them, and talks only to WhatsApp and, once a day, to GitHub to see
whether there is a newer version. The AI app
you use it with sends what it reads to its model provider, the same as anything else you show it.

Your AI can send files only from Desktop, Documents, Downloads, Pictures, Movies, Videos, Music and the temporary
folder, and never files or folders whose name starts with a dot, so a message crafted to trick it cannot reach your
SSH keys or app settings. It can still send any ordinary file in those folders, so keep secrets out of them. To allow
more folders,
set `CHATWIRE_FILES` to a list of full paths (separated by `:`, or `;` on Windows) in the app's MCP settings.
To let an app only read, set `CHATWIRE_READ_ONLY=1` in its MCP settings: it then gets no tool that sends or changes
anything. Both settings apply to the app that sets them, even when several apps share Chatwire.

## Updates

When a newer version is out, `whatsapp_status` tells your AI app, and `chatwire update` installs it after checking it
against the release checksums. Your AI apps use it from their next session. To turn off the daily check, set
`CHATWIRE_NO_UPDATE_CHECK=1`.

## Uninstall

1. `chatwire setup --remove` takes Chatwire out of your AI apps.
2. On your phone, open WhatsApp's Linked devices and log Chatwire out.
3. Delete the `chatwire` folder that holds your messages, media and keys: `~/Library/Application Support/chatwire` on
   macOS, `~/.config/chatwire` on Linux, `%AppData%\chatwire` on Windows.
4. Delete the program: `~/.local/bin/chatwire`, or the `%LOCALAPPDATA%\Programs\chatwire` folder on Windows.

## Be aware

WhatsApp's Terms of Service do not allow unofficial clients. Using Chatwire could get your WhatsApp account
restricted or banned. Chatwire paces its sends to behave like a person, but the risk is yours.

Chatwire does not take calls. It answers call signals the way a computer without calling does, so your phone keeps
ringing as usual. If calls on your phone ever get stuck at "Connecting", unlink Chatwire in WhatsApp's Linked devices
and [open an issue](https://github.com/PeterStoica/chatwire/issues).

## Built in Go

Chatwire is written in Go from scratch, including the WhatsApp protocol, its encryption and linking. It uses no C
code, not even for its database, so one machine builds it for macOS, Linux and Windows on both Intel and ARM, and
each build is a single file of about 18 MB with nothing to install next to it.

## Build from source

With Go 1.27 or newer:

```sh
go install github.com/PeterStoica/chatwire/cmd/chatwire@latest
```

Or from a clone: `go build ./cmd/chatwire` and `go test ./...`. See [CONTRIBUTING.md](CONTRIBUTING.md) to help, and
[SECURITY.md](SECURITY.md) to report a security problem.

## Ideas, bugs and questions

Everyone is welcome to open an [issue](https://github.com/PeterStoica/chatwire/issues) with ideas, bugs, questions or
anything that felt harder than it should. Every issue is read. Please leave out phone numbers, names and message text.

## License

Apache License 2.0, see [LICENSE](LICENSE) and [NOTICE](NOTICE). Copyright 2026 QA DNA.
Built by [Peter Stoica](https://github.com/PeterStoica), founder of [QA DNA](https://www.qadna.co).
