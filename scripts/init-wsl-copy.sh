#!/usr/bin/env bash
# One-time bootstrap: create ~/artex from the Windows copy.
# Run from WSL (any distro that can reach /mnt/c/).
set -euo pipefail

bash /mnt/c/Users/error/Desktop/ARTEX-main/ARTEX-main/scripts/sync-to-wsl.sh ~/artex

echo "WSL copy ready at ~/artex"
echo "To build: cd ~/artex && bash scripts/install-wsl-parrot.sh"
