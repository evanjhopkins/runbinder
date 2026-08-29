#!/usr/bin/env sh

set -eu

script_dir=$(CDPATH= cd "$(dirname "$0")" && pwd)
project_dir=$(dirname "$script_dir")

if [ -n "${GOBIN:-}" ]; then
	install_dir=$GOBIN
else
	install_dir="$(go env GOPATH)/bin"
fi

mkdir -p "$install_dir"
install_dir=$(CDPATH= cd "$install_dir" && pwd)
executable_path="$install_dir/runbinderd"
temporary_path=$(mktemp "${TMPDIR:-/tmp}/runbinderd.XXXXXX")

cleanup() {
	rm -f "$temporary_path"
}
trap cleanup 0 1 2 3 15

cd "$project_dir"
go build -o "$temporary_path" ./cmd/runbinder
mv "$temporary_path" "$executable_path"

printf 'Installed development executable: %s\n' "$executable_path"
