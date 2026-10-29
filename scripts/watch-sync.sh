#!/usr/bin/env bash
# Watch WSL copy and sync to Windows on any file change.
# Requires: inotify-tools  (sudo apt-get install inotify-tools)
# Run from WSL: bash ~/artex/scripts/watch-sync.sh [WSL_SRC]
set -euo pipefail

WSL_SRC="${1:-$HOME/artex}"
DEST="/mnt/c/Users/error/Desktop/ARTEX-main/ARTEX-main"

if ! command -v inotifywait &>/dev/null; then
  echo "ERROR: inotifywait not found. Install with: sudo apt-get install inotify-tools" >&2
  exit 1
fi

echo "Watching $WSL_SRC → $DEST (Ctrl-C to stop)"

while inotifywait -r -e modify,create,delete,move \
  --exclude '(node_modules|\.git|\.next|web/out)' \
  "$WSL_SRC" 2>/dev/null; do
  rsync -a --delete \
    --exclude='.git/' \
    --exclude='node_modules/' \
    --exclude='web/.next/' \
    --exclude='web/out/' \
    --exclude='data/' \
    --exclude='*.log' \
    "$WSL_SRC/" "$DEST/"
  echo "[$(date +%H:%M:%S)] Synced"
done
