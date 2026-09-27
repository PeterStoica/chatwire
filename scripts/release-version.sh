#!/bin/sh
set -eu
if [ "${GITHUB_REF_TYPE:-}" = tag ]; then
	echo "tag=${GITHUB_REF_NAME}"
	exit 0
fi
message=$(git log -1 --format='%s%n%b' "${1:-HEAD}")
version=$(printf '%s\n' "$message" | grep -oE 'release/v[0-9]+\.[0-9]+\.[0-9]+' | head -n 1 | sed 's#^release/##' || true)
if [ -z "$version" ]; then
	version=$(printf '%s\n' "$message" | grep -oE '[Rr]elease v[0-9]+\.[0-9]+\.[0-9]+' | head -n 1 | sed 's#^[Rr]elease ##' || true)
fi
if [ -z "$version" ]; then
	last=$(git tag -l 'v[0-9]*.[0-9]*.[0-9]*' --sort=-v:refname | head -n 1)
	if [ -z "$last" ]; then
		version=v0.1.0
	else
		major=$(echo "$last" | cut -d. -f1)
		minor=$(echo "$last" | cut -d. -f2)
		patch=$(echo "$last" | cut -d. -f3)
		version="$major.$minor.$((patch + 1))"
	fi
fi
if git rev-parse -q --verify "refs/tags/$version" >/dev/null; then
	echo "tag="
	echo "$version already exists; nothing to release" >&2
	exit 0
fi
echo "tag=$version"
