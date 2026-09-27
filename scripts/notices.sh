#!/bin/sh
set -eu
out=internal/notices/notices.txt
modules=$(for goos in darwin linux windows; do
	GOOS=$goos go list -deps -f '{{with .Module}}{{if not .Main}}{{.Path}} {{.Dir}}{{end}}{{end}}' ./cmd/chatwire
done | sort -u)
{
	cat NOTICE
	printf '\nChatwire is licensed under the Apache License, Version 2.0:\n\n'
	cat LICENSE
	printf '\n\nChatwire includes the following software, under these licences.\n'
	printf '%s\n' "$modules" | while read -r path dir; do
		for file in "$dir"/*; do
			name=${file##*/}
			case "$name" in
			LICENSE* | LICENCE* | COPYING* | NOTICE* | PATENTS* | *LICENSE-3RD-PARTY*) ;;
			*) continue ;;
			esac
			printf '\n== %s/%s\n\n' "$path" "$name"
			cat "$file"
		done
	done
} >"$out"
