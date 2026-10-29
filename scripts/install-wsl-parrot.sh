#!/usr/bin/env bash
# ARTEX installer for WSL Parrot OS (Parrot Security / Home)
# Run from the ARTEX project root:
#   bash scripts/install-wsl-parrot.sh
#
# What this does:
#   1. Installs Go 1.26.3 from golang.org (Parrot repos carry older versions)
#   2. Installs/upgrades Node.js 22 via NodeSource (Next.js 16 requires >= 20)
#   3. Installs PostgreSQL 16 and creates the artex database + role
#   4. Installs rsync (required by build.sh for the static-asset sync step)
#   5. Builds the frontend (Next.js static export)
#   6. Compiles the ARTEX binary with the embedded UI (CGO_ENABLED=0, -tags embedui)
#   7. Writes a config.json using the DB credentials you enter
#   8. Prints the access URL and the next step (run-artex.sh)
set -euo pipefail
cd "$(cd "$(dirname "$0")/.." && pwd)"  # always run from project root

# ── helpers ───────────────────────────────────────────────────────────────────
info()  { printf '\033[36m[*]\033[0m %s\n' "$*"; }
ok()    { printf '\033[32m[+]\033[0m %s\n' "$*"; }
warn()  { printf '\033[33m[!]\033[0m %s\n' "$*" >&2; }
die()   { printf '\033[31m[x]\033[0m %s\n' "$*" >&2; exit 1; }
ask()   { local p="$1" d="${2:-}" a; read -rp "$p${d:+ [$d]}: " a; echo "${a:-$d}"; }
rand()  { head -c 18 /dev/urandom | base64 | tr -dc 'A-Za-z0-9' | head -c 24; }
need()  { command -v "$1" >/dev/null 2>&1 || die "'$1' not found — install it and retry"; }

# ── sanity: must run inside WSL ───────────────────────────────────────────────
if ! grep -qi microsoft /proc/version 2>/dev/null; then
  warn "This script is designed for WSL. Continuing anyway, but results may differ on bare Parrot."
fi

# ── 1. Go 1.26.3 ─────────────────────────────────────────────────────────────
GO_VERSION="1.26.3"
GO_TARBALL="go${GO_VERSION}.linux-amd64.tar.gz"
GO_URL="https://go.dev/dl/${GO_TARBALL}"
GO_INSTALL_DIR="/usr/local"
GOBIN_PATH="${GO_INSTALL_DIR}/go/bin"

install_go() {
  info "Downloading Go ${GO_VERSION}..."
  TMP_GO=$(mktemp -d)
  curl -fsSL -o "${TMP_GO}/${GO_TARBALL}" "${GO_URL}"
  info "Installing Go ${GO_VERSION} to ${GO_INSTALL_DIR}..."
  sudo rm -rf "${GO_INSTALL_DIR}/go"
  sudo tar -C "${GO_INSTALL_DIR}" -xzf "${TMP_GO}/${GO_TARBALL}"
  rm -rf "${TMP_GO}"

  # Persist to /etc/profile.d so every future shell and sudo -i inherits it.
  cat > /tmp/artex-go.sh <<'GOENV'
export PATH="/usr/local/go/bin:$HOME/go/bin:$PATH"
GOENV
  sudo mv /tmp/artex-go.sh /etc/profile.d/artex-go.sh
  export PATH="${GOBIN_PATH}:${HOME}/go/bin:${PATH}"
  ok "Go $(go version) installed"
}

need_go_version() {
  # Returns 0 if go is present and meets the minimum minor version requirement.
  local min_minor=26
  if ! command -v go >/dev/null 2>&1; then return 1; fi
  # go version output: "go version go1.26.3 linux/amd64"
  local ver; ver=$(go version | grep -oP 'go\K[0-9]+\.[0-9]+')
  local major minor
  major=$(echo "$ver" | cut -d. -f1)
  minor=$(echo "$ver" | cut -d. -f2)
  [[ "$major" -gt 1 ]] && return 0
  [[ "$major" -eq 1 && "$minor" -ge "$min_minor" ]] && return 0
  return 1
}

if need_go_version; then
  ok "Go $(go version) already satisfies >= 1.${min_minor:-26}"
else
  install_go
fi
export PATH="${GOBIN_PATH}:${HOME}/go/bin:${PATH}"

# ── 2. Node.js 22 via NodeSource ─────────────────────────────────────────────
need_node_version() {
  if ! command -v node >/dev/null 2>&1; then return 1; fi
  local major; major=$(node -e 'process.stdout.write(process.version.replace(/^v/, "").split(".")[0])')
  [[ "$major" -ge 22 ]] && return 0
  return 1
}

if need_node_version; then
  ok "Node.js $(node --version) already satisfies >= 22"
else
  info "Installing Node.js 22 via NodeSource..."
  need curl
  need sudo
  curl -fsSL https://deb.nodesource.com/setup_22.x | sudo -E bash -
  sudo apt-get install -y nodejs
  ok "Node.js $(node --version) installed"
fi
need npm

# ── 3. apt packages: PostgreSQL 16, rsync ─────────────────────────────────────
info "Installing system packages (postgresql-16, rsync)..."
sudo apt-get update -qq
sudo apt-get install -y postgresql-16 postgresql-client-16 rsync 2>/dev/null || \
  sudo apt-get install -y postgresql postgresql-client rsync
ok "System packages ready"

# ── 4. PostgreSQL: create role + database ────────────────────────────────────
DB_USER="artex"
DB_NAME="artex"
DB_PASS="${ARTEX_DB_PASS:-$(rand)}"

info "Setting up PostgreSQL..."

# Start the cluster if not running (WSL does not auto-start services).
if ! sudo -u postgres psql -c '\q' >/dev/null 2>&1; then
  # Parrot/Debian cluster management
  PG_VERSION=$(pg_lsclusters | awk 'NR>1{print $1; exit}' 2>/dev/null || echo "16")
  sudo pg_ctlcluster "${PG_VERSION}" main start 2>/dev/null || \
    sudo service postgresql start
fi

# Create role (idempotent).
sudo -u postgres psql -tc "SELECT 1 FROM pg_roles WHERE rolname='${DB_USER}'" | grep -q 1 && \
  ok "PostgreSQL role '${DB_USER}' already exists" || \
  sudo -u postgres psql -c "CREATE ROLE ${DB_USER} LOGIN PASSWORD '${DB_PASS}';"

# Set the password even if the role existed (so config.json stays consistent).
sudo -u postgres psql -c "ALTER ROLE ${DB_USER} WITH PASSWORD '${DB_PASS}';"

# Create database (idempotent).
sudo -u postgres psql -tc "SELECT 1 FROM pg_database WHERE datname='${DB_NAME}'" | grep -q 1 && \
  ok "PostgreSQL database '${DB_NAME}' already exists" || \
  sudo -u postgres psql -c "CREATE DATABASE ${DB_NAME} OWNER ${DB_USER};"

ok "PostgreSQL role '${DB_USER}' + database '${DB_NAME}' ready"
ARTEX_DB_PASS="${DB_PASS}"

# ── 5. Write config.json ──────────────────────────────────────────────────────
cat > config.json <<JSON
{
  "database": {
    "host": "127.0.0.1",
    "port": 5432,
    "user": "${DB_USER}",
    "password": "${ARTEX_DB_PASS}",
    "dbname": "${DB_NAME}",
    "sslmode": "disable"
  },
  "skill_dir": ""
}
JSON
ok "config.json written (password: ${ARTEX_DB_PASS})"

# ── 6. Build frontend (Next.js static export) ─────────────────────────────────
info "Building frontend static assets (this takes 1-3 minutes)..."
(cd web && npm ci)
(cd web && npm run build:static)
mkdir -p server/webui/dist
rsync -a --delete web/out/ server/webui/dist/
ok "Frontend built and synced to server/webui/dist/"

# ── 7. Compile ARTEX binary ───────────────────────────────────────────────────
info "Compiling ARTEX binary (CGO_ENABLED=0, embedded UI)..."
CGO_ENABLED=0 go build \
  -tags embedui \
  -trimpath \
  -ldflags "-s -w -buildid=" \
  -o artex \
  ./cmd/artex
ok "Binary compiled: $(pwd)/artex  ($(du -sh artex | cut -f1))"

# ── 8. Done ───────────────────────────────────────────────────────────────────
echo
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
ok "ARTEX installed successfully"
echo
echo "  DB password (save this): ${ARTEX_DB_PASS}"
echo "  Config file:             $(pwd)/config.json"
echo "  Binary:                  $(pwd)/artex"
echo
echo "  Next — set your LLM key, then run:"
echo "    export ANTHROPIC_API_KEY='sk-ant-...'"
echo "    bash scripts/run-artex.sh"
echo
echo "  Web UI will be at:  http://localhost:8787"
echo "  MITM proxy at:      127.0.0.1:8788"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
