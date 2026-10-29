#!/usr/bin/env bash
set -euo pipefail
cd "$(cd "$(dirname "$0")/.." && pwd)"
export PATH="/usr/local/go/bin:$HOME/go/bin:$PATH"
service postgresql status >/dev/null 2>&1 || service postgresql start
set -a; [ -f .env ] && source .env; set +a
exec ./artex serve -addr :8787 -proxy 127.0.0.1:8788
