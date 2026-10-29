#!/usr/bin/env bash
# Push Windows copy → WSL copy.
# Run from WSL: bash /mnt/c/Users/error/Desktop/ARTEX-main/ARTEX-main/scripts/sync-to-wsl.sh [WSL_DEST]
set -euo pipefail

WSL_DEST="${1:-$HOME/artex}"
SRC="/mnt/c/Users/error/Desktop/ARTEX-main/ARTEX-main"

rsync -av --delete \
  --exclude='.git/' \
  --exclude='node_modules/' \
  --exclude='web/.next/' \
  --exclude='web/out/' \
  --exclude='data/' \
  --exclude='*.log' \
  "$SRC/" "$WSL_DEST/"

echo "Synced $SRC → $WSL_DEST"
