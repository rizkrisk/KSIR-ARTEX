# Running ARTEX on WSL Parrot OS

ARTEX is an autonomous AI penetration-testing platform. Parrot Security OS is an ideal host because it ships with the offensive tools (nmap, sqlmap, ffuf, etc.) that ARTEX's skills call. These instructions assume WSL 2 with a Parrot OS distro already installed and working on Windows 11.

---

## Prerequisites (Windows side)

- WSL 2 enabled (`wsl --install` or `wsl --set-default-version 2`)
- Parrot Security or Parrot Home installed as a WSL distro
- Project files at `C:\Users\error\Desktop\ARTEX-main\ARTEX-main` (already the case)

---

## Step 1: Open your Parrot WSL shell

```powershell
# In Windows Terminal or PowerShell:
wsl -d Parrot
```

Inside the shell, navigate to the project. WSL mounts Windows drives at `/mnt/c/`:

```bash
cd /mnt/c/Users/error/Desktop/ARTEX-main/ARTEX-main
```

---

## Step 2: One-shot install (recommended)

Run the installer. It handles everything non-interactively:

```bash
bash scripts/install-wsl-parrot.sh
```

What the installer does, in order:

| Step | Detail |
|------|--------|
| Go 1.26.3 | Downloads from `https://go.dev/dl/`, installs to `/usr/local/go`. Parrot repos carry Go 1.21 or older — too old. |
| Node.js 22 | Installs via NodeSource (`setup_22.x`). Next.js 16 requires Node ≥ 20. |
| PostgreSQL 16 | `apt-get install postgresql-16`. Creates role `artex` + database `artex`. |
| rsync | Required by `build.sh` to sync `web/out/` into `server/webui/dist/`. |
| Frontend build | `cd web && npm ci && npm run build:static` (Next.js static export). Takes 1–3 minutes. |
| Go binary | `CGO_ENABLED=0 go build -tags embedui ./cmd/artex`. No CGO, no cgo-linked SQLite — pure Go (modernc/sqlite). |
| config.json | Writes DB credentials to `config.json` in the project root. |

The installer prints the randomly generated PostgreSQL password at the end. **Save it.**

---

## Step 3: Set your LLM API key

ARTEX needs an LLM to run agents. Set the key before launching:

```bash
# Anthropic Claude (recommended):
export ANTHROPIC_API_KEY="sk-ant-YOUR_KEY_HERE"

# — or — OpenAI:
export OPENAI_API_KEY="sk-YOUR_KEY_HERE"
export ARTEX_LLM_PROVIDER="openai"
export ARTEX_LLM_MODEL="gpt-4o"

# — or — local Ollama (Windows host running Ollama):
export ARTEX_LLM_PROVIDER="ollama"
export ARTEX_LLM_MODEL="llama3.1"
# Use the Windows host IP that WSL can reach:
export ARTEX_LLM_BASE_URL="http://$(ip route show default | awk '{print $3}'):11434"
```

To persist across WSL restarts, add the export lines to `~/.bashrc`.

---

## Step 4: Launch ARTEX

```bash
bash scripts/run-artex.sh
```

The script:
1. Verifies the `artex` binary exists.
2. Starts PostgreSQL if not running (WSL does not auto-start services).
3. Delegates to `start.sh`, which restarts the binary automatically if it crashes.

You will see:

```
    _    ____ _____ _______  __
   / \  |  _ \_   _| ____\ \/ /
  ...
  version dev  ·  linux/amd64  ·  listening on :8787
```

Open `http://localhost:8787` in a Windows browser. The UI is embedded in the binary — no separate web server needed.

---

## Port map

| Port | Purpose | Accessible from |
|------|---------|----------------|
| `8787` | ARTEX web UI + JSON API | `localhost:8787` (Windows browser) |
| `8788` | MITM traffic recording proxy | `127.0.0.1:8788` (WSL-local only) |

To expose port 8787 to other machines on your LAN, edit `run-artex.sh` and change:
```bash
ARTEX_ADDR="${ARTEX_ADDR:-:8787}"
```
to:
```bash
ARTEX_ADDR="${ARTEX_ADDR:-0.0.0.0:8787}"
```

---

## MITM proxy (traffic recording)

The proxy on `127.0.0.1:8788` records HTTP/HTTPS traffic from your target application. To intercept HTTPS you must trust ARTEX's generated CA certificate in your browser or OS trust store.

The CA cert is generated automatically on first run and stored in the `data/` directory next to the binary:

```bash
# After first run:
ls data/  # contains artex.db, graph.db, jwt.key, and the proxy CA cert
```

---

## Skills directory

ARTEX reads skills from `skills/` in the project root by default. The Parrot OS tooling (nmap, sqlmap, nuclei, ffuf, httpx, etc.) is already on PATH inside WSL, so skills that shell out to those tools work without extra setup.

To use a custom skills directory:
```bash
export ARTEX_SKILL_DIR="/opt/my-skills"
```

---

## Starting PostgreSQL manually (if needed)

WSL does not run `systemd` by default, so PostgreSQL does not auto-start. The run script handles this, but you can do it manually:

```bash
# Parrot/Debian cluster management:
sudo pg_ctlcluster 16 main start

# Or the legacy service wrapper:
sudo service postgresql start
```

---

## Updating ARTEX

Pull the new source, then rebuild:

```bash
cd /mnt/c/Users/error/Desktop/ARTEX-main/ARTEX-main
git pull                      # or extract the new zip
ARTEX_SKIP_NPM_CI=0 bash build.sh --target linux/amd64
# → binary in dist/artex-linux-amd64/artex
cp dist/artex-linux-amd64/artex ./artex
bash scripts/run-artex.sh
```

---

## Troubleshooting

**`config file … not found — will only try the environment variable ARTEX_PG_DSN`**
The binary looks for `config.json` next to itself. Run from the project root, or set:
```bash
export ARTEX_CONFIG="/mnt/c/Users/error/Desktop/ARTEX-main/ARTEX-main/config.json"
```

**`no database config found`**
Either `config.json` is missing/malformed or `ARTEX_PG_DSN` is not set. The installer writes `config.json` for you. If you moved the binary, copy `config.json` alongside it.

**PostgreSQL connection refused**
WSL does not auto-start services. Run `sudo service postgresql start` or `sudo pg_ctlcluster 16 main start`.

**`npm run build:static` fails with Node version error**
Your Node is < 20. Reinstall with `curl -fsSL https://deb.nodesource.com/setup_22.x | sudo -E bash - && sudo apt-get install -y nodejs`.

**`go: command not found` after install**
Reload your shell: `source /etc/profile.d/artex-go.sh` or open a new terminal.

**Binary reports `embedui` build tag missing (no UI served)**
The binary was compiled without the `-tags embedui` flag, or `server/webui/dist/` was empty at build time. Re-run: `bash scripts/install-wsl-parrot.sh`.

---

## Windows ↔ WSL sync

### Recomendação: trabalhe direto no mount DrvFs

O caminho mais simples é **não manter duas cópias**. Acesse o projeto diretamente via mount DrvFs do WSL:

```bash
cd /mnt/c/Users/error/Desktop/ARTEX-main/ARTEX-main
```

É o mesmo diretório — sem sync, sem conflito. Use a cópia nativa em `~/artex` apenas se precisar de performance melhor (operações sobre muitos arquivos pequenos são lentas pelo DrvFs; compilações Go/npm pesadas se beneficiam da cópia nativa).

### Scripts disponíveis (quando a cópia nativa for necessária)

| Script | Direção | Comando |
|--------|---------|---------|
| `init-wsl-copy.sh` | One-time bootstrap | `bash /mnt/c/.../scripts/init-wsl-copy.sh` |
| `sync-to-wsl.sh` | Windows → WSL | `bash /mnt/c/.../scripts/sync-to-wsl.sh [DEST]` |
| `sync-from-wsl.sh` | WSL → Windows | `bash ~/artex/scripts/sync-from-wsl.sh [SRC]` |
| `watch-sync.sh` | WSL → Windows (auto) | `bash ~/artex/scripts/watch-sync.sh` |

**Criar a cópia nativa pela primeira vez:**

```bash
# No shell WSL:
bash /mnt/c/Users/error/Desktop/ARTEX-main/ARTEX-main/scripts/init-wsl-copy.sh
```

**Sync manual (Windows → WSL):**

```bash
bash /mnt/c/Users/error/Desktop/ARTEX-main/ARTEX-main/scripts/sync-to-wsl.sh
```

**Sync manual (WSL → Windows):**

```bash
bash ~/artex/scripts/sync-from-wsl.sh
```

**Auto-sync (WSL → Windows, requer `inotify-tools`):**

```bash
sudo apt-get install -y inotify-tools
bash ~/artex/scripts/watch-sync.sh
```

Todos os scripts excluem `.git/`, `node_modules/`, `web/.next/`, `web/out/`, `data/` e `*.log`.
