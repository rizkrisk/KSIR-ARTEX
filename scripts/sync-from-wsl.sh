#!/usr/bin/env bash
# Push WSL copy → Windows copy.
# Run from WSL: bash ~/artex/scripts/sync-from-wsl.sh [WSL_SRC]
set -euo pipefail

WSL_SRC="${1:-$HOME/artex}"
DEST="/mnt/c/Users/error/Desktop/ARTEX-main/ARTEX-main"

rsync -av --delete \
  --exclude='.git/' \
  --exclude='node_modules/' \
  --exclude='web/.next/' \
  --exclude='web/out/' \
  --exclude='data/' \
  --exclude='*.log' \
  "$WSL_SRC/" "$DEST/"

echo "Synced $WSL_SRC → $DEST"
