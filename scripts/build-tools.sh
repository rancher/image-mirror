#!/usr/bin/env bash

REPO_ROOT=$(git rev-parse --show-toplevel)
TOOLS_DIR="$REPO_ROOT/tools"
BIN_DIR="$REPO_ROOT/bin"
TOOLS_PATH="$BIN_DIR/image-mirror-tools"

rm -f "$TOOLS_PATH"
mkdir -p "$BIN_DIR"
cd "$TOOLS_DIR"
go build -o "$TOOLS_PATH"
