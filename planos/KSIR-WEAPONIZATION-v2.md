# KSIR ARTEX — Plano de Weaponização v2

> **Artefato:** Plano corrigido e executor-ready. Substitui `planos/KSIR-WEAPONIZATION.md` (v1).
> **Base auditada:** `planos/KSIR-AUDIT-COMPLEMENTO.md` (854 linhas, 88 complementos).
> **Data:** 2026-10-27
> **Stack confirmada:** Go 1.26.3 + Next.js embed + PostgreSQL 16 + norma SDK v0.4.4. Build `CGO_ENABLED=0`. Produção `python:3.12-slim-bookworm`. Host: Windows 11 x64, Parrot OS em WSL.

---

## BLOQUEIOS DO OPERADOR — RESPOSTAS

| # | Pergunta | Resposta | Impacto |
|---|---|---|---|
| B1 | API do painel C2? | **RAT Windows comum, sem API HTTP**. Sem endpoints de geração de implant, listagem de listeners, execução em sessão, upload/download. | Módulo #7 degrada para **Delivery Assist manual**. ARTEX entrega o binário RAT pela shell já obtida e passa controle ao operador via `ask_operator()`. Autonomia de pós-exploração não existe neste cenário. |
| B2 | Perfil de evasão negociável? | **Não**. RAT gera binário único. | Matriz `check_edr()` (§2.9 da auditoria) vira referência de decisão manual para o operador escolher quando/como usar o RAT. `c2_deliver_implant()` não tem parâmetro `profile`. |
| B3 | Primitiva de cleanup pós-operação? | **Não**. RAT não expõe isso. | Módulo #9 implementa cleanup via comandos shell diretos. Mais lento e mais ruidoso. Impacto declarado. |
| B4 | Reporter: Opção A (repurpose) ou B (desligar)? | **Opção A**: repurpose para curadoria de exfiltração. | `bind_finding_traffic` mantido como rastreamento de proveniência. Alinha com #1. |
| B5 | Desarme total do Intercept ou manter trilho de escopo? | **Manter trilho de escopo** como inegociável. Desarmar `deny`/`ask` ofensivos, mas preservar regras de escopo autorizado. | Única proteção contra ARTEX atingir infraestrutura fora do alvo. Recomendação da auditoria mantida. |
| B6 | CVE-2026-109618/109619 existem? | **Não verificado em fonte primária**. Vêm de `skills/php2shell/SKILL.md`. | Popular seed com flag `confidence = "provisional"`. Não usar em produção sem confirmar. |
| B7 | PoCs do php2shell criados do zero? | **Sim, criados do zero**. O "estado atual" do v1 era inteiramente fantasma. | Custo orçado explicitamente no módulo #5. |

---

## PONTAS SOLTAS TÉCNICAS — RESOLUÇÕES

| # | Ponta solta | Resolução |
|---|---|---|
| 1 | Hook do PolyEngine — tools retornam erro dentro de `Result`, não como `error` Go | Hook em `agent/toolcatalog.go` / `AugmentTools`. Ver §1.5 da auditoria. Detalhado no módulo #6. |
| 2 | Escopo do #6 muito estreito | Elevado para "replanejar abordagem": payload, ferramenta, vetor, ordem de ataque. |
| 3 | Camada determinística antes do LLM no #6 | Mutadores WAFFLED mecânicos primeiro. LLM só quando mutadores esgotarem sem sucesso. Corta token em ordem de magnitude. |
| 4 | Registro de tools novas em dois lugares | `agent/toolcatalog.go` **E** `agent/tools_insert.go:656` (WorkerTools) / `:687` (MainAgentTools) / `:715` (AllDomainTools). Todo módulo que cria tool precisa de ambos. |
| 5 | Interface `C2Backend` | Adaptada para RAT sem API: `Deliver(ctx, vector, binaryPath string) error`, `Stage(ctx, arch string) (string, error)`. Sem `RequestImplant`, `ListListeners`, `Exec`, `Cleanup` — RAT não expõe isso. |
| 6 | #8: separar responsabilidades de proxy | `proxygraph` = rotação de saída (IP/ASN/TLS fingerprint/pool). Forma do tráfego = responsabilidade separada em `proxygraph/traffic.go`. |
| 7 | #8: encadear através do MITM existente | `proxygraph` encadeia via MITM de egresso (`agent/worker.go:206-214`). Nunca em paralelo — quebra `traffic_*` e cadeia de evidência do #1 e #3b. |
| 8 | Domain fronting — AWS e Google desabilitaram | Usar serverless próprio (Cloudflare Workers, MITRE T1583.007) com tráfego similar. Fronting clássico removido do plano. |
| 9 | CVEs do #4 mal atribuídas | Corrigidas conforme tabela §1.11 da auditoria. Módulo #4 tem tabela correta. |
| 10 | Boot check para falha silenciosa de reseed | Adicionar verificação em `server/server.go` que loga `WARN` se `current_prompt_id` do worker não contiver marcadores KSIR. Transforma falha silenciosa em falha visível. |
| 11 | Teste de integração de evidência após #8 | Adicionado ao módulo #8: tarefa com captura ligada → tool via proxy rotativo → afirmar que `traffic_search` retorna o registro. |
| 12 | `RankArtifacts(taskID)` no #1 | Classificar por valor antes de empacotar: densidade de segredo, reutilizabilidade imediata. Dump de 40GB vale menos que 200 linhas de `web.config`. |
| 13 | Nó `identity` no asset graph | Schema: `{id bigint, type text = 'identity', email text, username text, source text, credential_ref bigint → task_credentials.id, tenant_domain text, created_at timestamptz}`. Adicionado ao módulo #10. |
| 14 | RAG vetorial entre workers | **Roadmap futuro** — não implementado no v2. Decisão explícita: colocar em `planos/ROADMAP-FUTURO.md`. Não omitir, não bloquear o v2. |

---

## TABELA-ÍNDICE DE MÓDULOS

| # | Módulo | Fase | Depende de | Complexidade |
|---|---|---|---|---|
| #0 | Desarme do Intercept + Constraints | 0 | — | ALTA |
| #0b | Migração de reseed de prompts | 0 | #0 | ALTA |
| #2 | Remover Rate Limiters | 1 | #0, #0b | BAIXA |
| #3 | Weaponized Exploitation (Workers) | 1 | #0, #0b | CRÍTICA |
| #3b | Repurpose do Reporter | 1 | #3 | MÉDIA |
| #6 | PolyEngine | 1 | #2, #3 | ALTA |
| #5 | PHP2Shell Cluster | 2 | #3, #6 | MÉDIA |
| #4 | DB + Exploits Recentes | 2 | #3 | ALTA |
| #1 | Exfiltração | 2 | #3b, #4 | ALTA |
| #8 | Grafo de Proxy Dinâmico | 3 | #1, #3b | ALTA |
| #7 | Delivery Assist C2 | 3 | #8 | MÉDIA |
| #9 | Movimentação Lateral + Priv-Esc | 3 | #7 | CRÍTICA |
| #10 | Superfície de Nuvem e Identidade | 3 | #4, #9 | ALTA |

---

## ORDEM DE EXECUÇÃO

```
Fase 0 — Desarme (bloqueante para todo o resto)
  #0  Intercept + Constraints
      Aresta: sem isso, #3 reescreve prompt mas tools continuam vetadas em runtime.
  #0b Migração de reseed
      Aresta: sem isso, #3 é código morto em DB existente.

Fase 1 — Fundação
  #2  Rate Limiters
      Aresta: #6 consome LLM por tentativa — com rate limiter ativo fica limitado a poucas mutações.
  #3  + #3b Worker + Reporter
      Aresta: #3b depende do novo ReporterDefaultPrompt criado em #3.
              #6 precisa dos gates removidos — PolyEngine gera variantes que o adversarial gate descartaria.
  #6  PolyEngine
      Aresta: antecipado da Fase 2. Depende de #2 (volume LLM) e de #3 (gates fora).

Fase 2 — Exploração
  #5  PHP2Shell Cluster
      Aresta: depende de #3 (worker sem gates) e de #6 (payloads polimórficos).
  #4  DB + Exploits
      Aresta: popula seed que #5 e #1 usam. Ordem dentro da fase: #4 antes de #1.
  #1  Exfiltração
      Aresta: depende de #3b (proveniência de artefato via reporter) e de #4 (exfil_artifacts schema).

Fase 3 — Evasão e Expansão
  #8  Grafo de Proxy
      Aresta: encadeia via MITM. Precisa de #1 funcionando para não quebrar proveniência.
  #7  Delivery Assist C2
      Aresta: #9 precisa da delivery infrastructure do #7 para chegar no host lateral.
  #9  Movimentação Lateral
      Aresta: roda através da shell entregue pelo #7.
  #10 Cloud + Identidade
      Aresta: paralelo com #9. Sem dependência direta de #9, mas usa tabelas de #4 e credentials de #9.
```

**Nota contra-intuitiva 1:** #6 foi antecipado para Fase 1. Razão: o hook do PolyEngine intercepta resultado de tool em `toolcatalog.go/AugmentTools` — que é o ponto onde o adversarial gate também opera. Se o gate ainda estiver ativo no #6, ele descarta as variantes geradas. E o #6 consome LLM adicional por tentativa, então o rate limiter (removido no #2) precisa sair antes.

**Nota contra-intuitiva 2:** #7 vem antes de #9 na Fase 3. Razão: movimentação lateral roda através da sessão (ou shell) entregue pelo módulo #7. Sem o mecanismo de delivery do #7, as tools do #9 não têm canal de execução remoto.

---

## MÓDULO #0 — Desarme do Intercept + Constraints

### Objetivo
Remover os dois gates de runtime que vetam tool calls independentemente do conteúdo do prompt: (a) regras `deny`/`ask` do subsistema `intercept/`, (b) injeção de `constraintBlock()` no system prompt do worker e do planner. Manter as regras de escopo autorizado como trilho inegociável (B5).

### Dependências
Nenhuma. Pré-requisito de todos os outros módulos.

### Arquivos

| AÇÃO | Caminho | O que muda |
|---|---|---|
| MODIFICAR | `server/server.go` | Seed de `intercept_rules` com lista vazia por padrão (não seed regras `deny`/`ask` ofensivas). |
| MODIFICAR | `agent/worker.go:446` | Condicionar `sysBody += constraintBlock(ts)` atrás de `wantConstraints()` — que já existe em `:75`. Adicionar função de override `SetConstraintInject(func() bool { return false })` e chamá-la na inicialização do worker no modo KSIR. |
| MODIFICAR | `agent/planner.go:412` | Mesmo tratamento: `sysBody += constraintBlock(ts)` condicional via `wantConstraints()`. |
| MODIFICAR | `intercept/prompt.go` | Mudar para modo "conselheiro": loga a avaliação, não bloqueia. Remover do caminho `PreToolUse` ou fazer retornar `allow` incondicionalmente para tools ofensivas KSIR. |
| MODIFICAR | `db/intercept.go` | Adicionar função `DisableOffensiveRules(db, agentIDs []string) error` — desativa regras `deny`/`ask` para `["worker", "pentest", "auto"]`, preserva regras de escopo. |

### Símbolos
- `agent/worker.go:75` — `injectConstraints` flag
- `agent/worker.go:145` — `SetConstraintInject(fn)`
- `agent/worker.go:153` — `wantConstraints()` retorna `true` se resolver for `nil`
- `agent/worker.go:446` — `sysBody += constraintBlock(ts)` — alvo da condicional
- `agent/planner.go:37` — `injectConstraints` flag no planner
- `agent/planner.go:104` — `SetConstraintInject(fn)` no planner
- `agent/planner.go:112` — `wantConstraints()` no planner
- `agent/planner.go:412` — `sysBody += constraintBlock(ts)` no planner — alvo da condicional
- `agent/constraints.go:9` — `constraintBlock(ts)` — função que renderiza constraints
- `intercept/intercept.go:5-11` — docstring com ações `allow`/`deny`/`ask`
- `agent/tools.go:1702-1721` — tool `set_constraints` (decidir destino: manter ou remover)
- `agent/goals.go:21-22` — extração de constraints antes de decompor goals (decidir destino)

### Contratos

```go
// DisableOffensiveRules desativa regras deny/ask para os agentIDs informados.
// Preserva regras de tipo "scope" (escopo autorizado inegociável, B5).
func DisableOffensiveRules(db *DB, agentIDs []string) error

// NoopConstraintResolver retorna sempre false para wantConstraints.
var NoopConstraintResolver = func() bool { return false }
```

### Testes que quebram

| Arquivo de teste | O que verifica | Novo marcador |
|---|---|---|
| `db/intercept_detail_test.go` | CRUD de detalhe de regra | Manter — não toca o schema |
| `db/intercept_execution_test.go` | Log de execuções | Manter |
| `db/intercept_filter_test.go` | Filtro de regras | Manter |
| `db/intercept_seed_test.go` | Seed de regras de fábrica | Ajustar: afirmar que seed de fábrica não contém regras `deny`/`ask` para agentes workers |
| `db/asset_intercept_match_test.go` | Matching de asset | Manter |
| `server/intercept_detail_test.go` | API de detalhe | Manter |
| `server/intercept_filter_test.go` | API de filtro | Manter |
| `server/intercept_live_test.go` | Fluxo live | Manter — escopo preservado |
| `server/intercept_review_test.go` | Revisão pendente | Manter |

**Novo teste a criar:** `db/intercept_seed_test.go` — `TestKSIRSeedHasNoOffensiveDenyAsk` verifica que o seed de fábrica KSIR não contém `action IN ('deny','ask')` para agentes `worker`/`pentest`/`auto`.

### Critério de aceite
```bash
# Criar tarefa, worker executa tool ofensiva qualquer sem ficar pendurado em intercept_pending
go test ./db/... ./server/... ./intercept/... -run TestKSIRSeed -v
# esperado: PASS
# verificar manualmente: nenhum registro em intercept_pending após tool call de worker
```

### Rollback
Nenhuma migração de dados. Reverter os 5 arquivos modificados via git checkout. O subsistema de intercept volta ao estado original. Regras de intercept no DB persistem intactas porque `DisableOffensiveRules` não deleta — apenas desativa com flag.

---

## MÓDULO #0b — Migração de Reseed de Prompts

### Objetivo
Garantir que o novo prompt ofensivo KSIR seja ativado em bancos já populados. O seed é *first-insert-only* (`db/config.go:773-777`) — reescrever a const Go não muda nada num DB existente. Exige migração no padrão de `server/orchestration.go:577-690`.

### Dependências
#0 — Intercept desarmado. A migração de prompt faz sentido executar depois, para não ativar o prompt ofensivo com gates ainda ativos.

### Arquivos

| AÇÃO | Caminho | O que muda |
|---|---|---|
| MODIFICAR | `agent/worker.go:242` | Reescrever `workerDefaultTmpl` com doutrina ofensiva KSIR (seção [A] apenas — [B] e [C] são code-owned e permanecem). |
| MODIFICAR | `agent/promptcatalog.go:104` | Reescrever `DefaultAssistantPrompt` — remover referências ao workflow defensivo. |
| MODIFICAR | `agent/promptcatalog.go:110` | `ReporterDefaultPrompt` — reescrito no módulo #3b. Aqui: apenas registrar que precisa de reseed. |
| MODIFICAR | `agent/retester.go:4` | `RetesterDefaultPrompt` — reescrever: remover marcadores adversariais (quebra `agent/adversarial_gate_seed_test.go:30`). |
| MODIFICAR | `server/server.go` (~linha 223) | Chamar as funções de reseed junto das outras migrações de `server/orchestration.go`. |
| CRIAR | `server/reseed_ksir.go` | `reseedKSIRWorkerPrompt()`, `reseedKSIRPentestPrompt()`, `reseedKSIRReporterPrompt()`, `reseedKSIRRetesterPrompt()` — seguindo o padrão de `server/orchestration.go:577-690`. Flag: `ksir_weaponized_prompts_v1`. |

### Símbolos
- `agent/worker.go:237` — comentário que delimita seção [A] editável
- `agent/worker.go:242` — `const workerDefaultTmpl = ...`
- `agent/worker.go:326` — `renderSystem(...)` — onde [B] e [C] são anexados (não tocar)
- `agent/promptcatalog.go:151` — `BuiltinPromptSeeds()` — tabela de seeds
- `agent/promptcatalog.go:156` — `"worker": workerDefaultTmpl`
- `db/config.go:766` — query que lê o prompt do banco: `SELECT p.template_text FROM agents a JOIN agent_prompts p ON p.id=a.current_prompt_id WHERE a.id=$1`
- `db/config.go:773-777` — `SeedPromptIfEmpty` — first-insert-only
- `server/orchestration.go:577` — comentário explicando o padrão de migração
- `server/assembly.go:202-218` — comentário sobre first-insert-only seed

### Contratos

```go
// reseedKSIRWorkerPrompt insere nova versão ofensiva do worker prompt e atualiza current_prompt_id.
// É idempotente: guarda de flag "ksir_weaponized_prompts_v1" impede re-execução.
func reseedKSIRWorkerPrompt(ctx context.Context, db *db.DB) error

// reseedKSIRPentestPrompt idem para o agente pentest.
func reseedKSIRPentestPrompt(ctx context.Context, db *db.DB) error

// reseedKSIRReporterPrompt idem para o reporter — será sobrescrito novamente no #3b.
func reseedKSIRReporterPrompt(ctx context.Context, db *db.DB) error

// reseedKSIRRetesterPrompt idem para o retester.
func reseedKSIRRetesterPrompt(ctx context.Context, db *db.DB) error
```

### Prompt ofensivo do worker (corpo da seção [A] a ser escrito em `workerDefaultTmpl`)

```
EXECUTOR OFENSIVO KSIR — Modo Weaponized Exploitation

Você é um executor de penetration testing autorizado. Recebe [UM INTENT] e sua função é:
explorar ativamente → extrair → persistir → reportar.

Prioridade de exploit: RCE > Injeção de Comando > SQLi→RCE > LFI→RCE > Auth Bypass > Info Disclosure.

Regras de operação:
- Sem gates adversariais: se payload funcionou, registre via report_finding e continue.
- Toda vuln confirmada deve ser explorada até exfiltração quando possível.
- Cadeias de exploit: encadear automaticamente PHP2Shell, SQL→RCE, LFI→RCE.
- Decisões autônomas de exploração: não pausar para aprovação humana exceto em escopo inválido.
- Registrar artefatos exfiltrados via exfil_data com path e tipo.
- Persistência oportunista: se houver vetor de persistência, explorar e registrar.

Limites não negociáveis:
- Não atacar hosts/domínios fora do escopo autorizado desta tarefa.
- Se não houver escopo definido, perguntar via ask_operator antes de qualquer ação externa.
```

**Nota:** Seções [B] (`workerTrafficBlock`) e [C] (`workerArtifactSpec`) são code-owned e anexadas automaticamente por `workerSystem()` — não reescrever, não remover. `workerArtifactSpec` é **dependência positiva** do módulo #1: força artefatos para o workdir da tarefa que o módulo #1 varre.

**Boot check (ponta solta #10):** Adicionar em `server/server.go` após as migrações:
```go
if !strings.Contains(currentWorkerPrompt, "EXECUTOR OFENSIVO KSIR") {
    log.Warn("KSIR: worker prompt não contém marcador ofensivo — reseed não aplicado ou DB desatualizado")
}
```

### Testes que quebram

| Arquivo | Marcador atual | Novo marcador KSIR |
|---|---|---|
| `agent/adversarial_gate_seed_test.go:30` | Adversarial gate nos prompts | `TestPromptSeedsCarryKSIRDoctrine` — afirma `"EXECUTOR OFENSIVO KSIR"` em `workerDefaultTmpl` |
| `agent/finding_quality_seed_test.go:31` | Measured impact em `ReporterDefaultPrompt` | Reescrever para marcador novo do #3b |
| `agent/finding_dedup_seed_test.go:23` | Dedup em `ReporterDefaultPrompt` | Reescrever para marcador novo do #3b |
| `agent/prompt_injection_seed_test.go` | Hardening anti-injeção | Manter — proteção contra injeção do alvo é vantajosa. Verificar se marcador ainda existe no novo template. |
| `agent/operator_handoff_seed_test.go` | Handoff ao operador em falhas | Ajustar: manter só para falhas de escopo (pede `ask_operator` quando host fora de escopo) |
| `agent/deadline_contract_seed_test.go` | Contrato de prazo | Avaliar: se o deadline contract interferir com exploração longa, remover o marcador do template |

**Decisão de reescrita dos testes:** Trocar nome da função de teste, não deletar o teste. Os seed tests são especificação executável — sem eles não há garantia de que a migração de reseed funcionou. `TestPromptSeedsCarryKSIRDoctrine` é o substituto que afirma os novos marcadores.

### Critério de aceite
```bash
# Seed tests novos passam
go test ./agent/... -run TestPromptSeedsCarryKSIRDoctrine -v
# Boot check loga OK (não WARN) após o reseed
# Verificar no log: ausência de "reseed não aplicado"
go test ./server/... -run TestReseedKSIRIdempotent -v
# esperado: PASS (segunda execução da migração não cria nova versão do prompt)
```

### Rollback
Histórico de versão em `agent_prompts` preservado por design (padrão de `orchestration.go`). Rollback: via System Settings → Agent → Worker → selecionar versão anterior do prompt (UI existente). Flag `ksir_weaponized_prompts_v1` pode ser deletada do Settings para forçar re-execução.

---

## MÓDULO #2 — Remover Rate Limiters

### Objetivo
Liberar o budget de LLM e de notificações para operação ofensiva de alto volume. O PolyEngine (#6) e o worker ofensivo vão consumir muito mais tokens que o uso defensivo original.

### Dependências
#0, #0b — fundação precisa estar pronta.

### Arquivos

| AÇÃO | Caminho | O que muda |
|---|---|---|
| MODIFICAR | `server/notifier.go:50` | `notifyUnlimitedBurstPerTick = 50` → `notifyUnlimitedBurstPerTick = 9999` (não zerar — valor 0 pode quebrar a lógica de token bucket; usar valor alto) |
| MODIFICAR | `notify/channel.go` | `DefaultRatePerMin()` — retornar 0 como padrão (sem limite) |
| MODIFICAR | `agent/provider.go:376` | Não setar `lc.RateLimit` (deixar nil) — remover `lc.RateLimit = &llm.RateLimit{...}` |
| MODIFICAR | `web/src/app/(main)/system/llm/page.tsx:631-652` | Remover campos UI de "Rate limit per second" e "Rate limit per minute" |
| MODIFICAR | `web/src/app/(main)/system/notify/page.tsx` | Remover campos UI de rate limit por canal |

### Símbolos
- `server/notifier.go:46-50` — bloco `notifyUnlimitedBurstPerTick`
- `server/notifier.go:529` — `return min(want, notifyUnlimitedBurstPerTick)` — onde o cap é aplicado
- `notify/channel.go:35-38` — interface `DefaultRatePerMin()` e implementações
- `agent/provider.go:376` — `lc.RateLimit = &llm.RateLimit{PerSecond: c.RatePerSecond, PerMinute: c.RatePerMinute}`
- `server/notify_api.go:227-238` — validação de `rate_per_min >= 0` (verificar se muda com DefaultRatePerMin retornando 0)

### Contratos
Sem novos contratos. Mudança de constante e remoção de assignment.

### Testes que quebram

| Arquivo | Linha | O que afirma | Correção |
|---|---|---|---|
| `server/notify_api_test.go:917-918` | `notifyUnlimitedBurstPerTick` | Afirma retorno exato da constante | Atualizar valor esperado para 9999 |

### Critério de aceite
```bash
go test ./server/... -run TestNotify -v
# esperado: PASS com novo valor de burst
go test ./agent/... -run TestProvider -v
# verificar que provider não seta RateLimit
```

### Rollback
Reverter os 5 arquivos via git. Sem migração de dados.

---

## MÓDULO #3 — Weaponized Exploitation (Workers)

### Objetivo
Transformar o worker de "executor defensivo com gates de qualidade" em "executor ofensivo autônomo". O conteúdo do prompt foi definido no módulo #0b. Este módulo trata das mudanças estruturais: remoção de gates em código, adição de tools ofensivas, modificação de `addFinding()`.

### Dependências
#0 (gates de runtime desarmados), #0b (novo prompt ativado no DB).

### Arquivos

| AÇÃO | Caminho | O que muda |
|---|---|---|
| MODIFICAR | `agent/finding_recorder.go` | Remover validação de `evidence_hint_id` obrigatório. Aceitar finding sem evidência vinculada. |
| MODIFICAR | `agent/finding_workflow.go:56` | Remover gate de verificação de tráfego antes de escrever o relatório (módulo #3b assume essa responsabilidade de forma diferente). |
| MODIFICAR | `agent/tools.go:1345` | `addFinding()` — remover validação do adversarial gate. Adicionar campos `exploit_chain text` e `exfil_path text` ao payload. |
| MODIFICAR | `agent/toolcatalog.go` | Registrar tools ofensivas novas. |
| MODIFICAR | `agent/tools_insert.go:656` | `WorkerTools()` — adicionar tools ofensivas. |
| CRIAR | `agent/tools_offensive.go` | Tools: `deploy_payload`, `establish_persistence`, `exfiltrate`, `lateral_move`, `dump_credentials`. |

### Símbolos
- `agent/tools.go:1345` — `addFinding()` (auditoria confirmou: 1345, não 1346 como dizia v1)
- `agent/tools.go:1405` — trigger automático do reporter após `report_finding`
- `agent/finding_workflow.go:32` — flag `finding_workflow_tools_v2_reporter`
- `agent/finding_workflow.go:56` — gate de verificação de tráfego
- `agent/tools.go:1375-1380` — `findingTrafficBindingEnabled()` — padrão de degradação silenciosa a ser seguido (não rejeitar, degradar)

### Contratos

```go
// agent/tools_offensive.go

// DeployPayloadArgs são os parâmetros da tool deploy_payload.
type DeployPayloadArgs struct {
    TargetURL   string `json:"target_url"`
    PayloadType string `json:"payload_type"` // webshell/reverse_shell/bind_shell
    PayloadBody string `json:"payload_body"`
    DeliveryMethod string `json:"delivery_method"` // upload/sqli/lfi/rce
}

// EstablishPersistenceArgs parâmetros de establish_persistence.
type EstablishPersistenceArgs struct {
    Method  string `json:"method"` // cron/registry/service/startup
    Command string `json:"command"`
    Target  string `json:"target"` // host ou processo alvo
}

// ExfiltrateArgs parâmetros de exfiltrate.
type ExfiltrateArgs struct {
    TargetPath string `json:"target_path"`
    Pattern    string `json:"pattern"`
    MaxSizeMB  int    `json:"max_size_mb"`
}

// LateralMoveArgs parâmetros de lateral_move.
type LateralMoveArgs struct {
    Target     string `json:"target"`
    Method     string `json:"method"` // wmi/winrm/smb/ssh/psexec/dcom
    Credential string `json:"credential_id"` // ref para task_credentials.id
}

// DumpCredentialsArgs parâmetros de dump_credentials.
// Nota: não cria tool duplicada. Chama saveTaskCredential() existente em agent/tools_creds.go:17.
type DumpCredentialsArgs struct {
    Method string `json:"method"` // lsass/sam/ntds/dpapi/kerberos/files
    Target string `json:"target"`
}
```

**Schema de finding atualizado** (adicionar colunas em `db/schema.sql`):
```sql
ALTER TABLE findings ADD COLUMN IF NOT EXISTS exploit_chain TEXT;
ALTER TABLE findings ADD COLUMN IF NOT EXISTS exfil_path TEXT;
```

### Testes que quebram

| Arquivo | Correção |
|---|---|
| `agent/finding_recorder_test.go:106` | Atualizar: finding sem evidence_hint_id deve ser aceito |
| `agent/finding_workflow_test.go` | Revisar: gates removidos não devem mais rejeitar |
| `server/finding_workflow_test.go:265` | `TestFindingWorkflowReporterBindsBeforeWritingReport` — será ajustado no #3b |

### Critério de aceite
```bash
go test ./agent/... -run TestOffensiveTools -v
# Tool deploy_payload registrada e invocável
go test ./agent/... -run TestAddFindingNoEvidence -v
# Finding sem evidence_hint_id aceito sem erro
CGO_ENABLED=0 go build -tags embedui -o artex ./cmd/artex
# Build limpo
```

### Rollback
Reverter os 6 arquivos. As colunas `exploit_chain` e `exfil_path` adicionadas ao schema são non-breaking (nullable) — podem permanecer sem prejudicar rollback funcional.

---

## MÓDULO #3b — Repurpose do Reporter

### Objetivo
Transformar o agente `reporter` de "escritor de relatório defensivo" em "agente de curadoria de exfiltração". Decisão: Opção A (B4). Mantém infraestrutura de trigger automático e proveniência via `bind_finding_traffic`.

### Dependências
#3 — novo prompt do worker ativado. O reporter é disparado pelo worker via `report_finding`.

### Arquivos

| AÇÃO | Caminho | O que muda |
|---|---|---|
| MODIFICAR | `agent/promptcatalog.go:110` | `ReporterDefaultPrompt` — reescrever para curadoria de exfiltração (ver prompt abaixo). |
| MODIFICAR | `agent/finding_workflow.go:77` | Atualizar guidance de auto-binding para curadoria, não relatório. |
| CRIAR | `server/reseed_ksir.go` | `reseedKSIRReporterPrompt()` — chama reseed do novo `ReporterDefaultPrompt` (já previsto no #0b). |

### Símbolos
- `agent/promptcatalog.go:106-110` — `ReporterDefaultPrompt`
- `agent/tools.go:1405` — trigger automático após `report_finding` — **manter**
- `server/finding_workflow.go:90` — `readers := '["worker","reporter"]'`
- `server/finding_workflow.go:100` — `bind_finding_traffic` vinculado para `["reporter"]` somente — **manter**
- `server/finding_workflow.go:103` — `AddAgentToToolBinding("reporter", ...)` — **manter**
- `agent/finding_workflow.go:32` — flag `finding_workflow_tools_v2_reporter` — **manter ativa**
- `agent/finding_workflow.go:77` — guidance de binding

### Novo `ReporterDefaultPrompt`

```
CURADOR DE EXFILTRAÇÃO KSIR

Você é o agente de curadoria. Disparado automaticamente após cada finding confirmado.
Sua função: enumerar e catalogar os artefatos extraídos, não escrever relatório.

Passos obrigatórios:
1. Verificar e vincular tráfego relevante via bind_finding_traffic (proveniência).
2. Enumerar artefatos no run dir do finding (arquivos, credenciais, dumps).
3. Extrair credenciais em formato utilizável: hash NTLM, cookie de sessão, token JWT/OAuth.
4. Classificar por valor: credencial > config com senha > dump de banco > source code > screenshot.
5. Popular exfil_artifacts para cada artefato encontrado com artifact_type e stored_path.
6. Retornar resumo: N artefatos catalogados, tipos, valor estimado.

Nunca escrever relatório Markdown. Nunca marcar finding como não confirmado por falta de tráfego.
KSIR_REPORTER_CURATION_V1
```

### Contratos
Sem novos contratos Go. O reporter usa as tools existentes (`traffic_search`, `traffic_get`, `traffic_blob`, `bind_finding_traffic`) e as novas de exfiltração do módulo #1 (`exfil_data`).

### Testes que quebram

| Arquivo | Novo marcador |
|---|---|
| `agent/finding_quality_seed_test.go:31` | `TestPromptSeedsCarryKSIRCurationMarker` — afirma `"KSIR_REPORTER_CURATION_V1"` em `ReporterDefaultPrompt` |
| `agent/finding_dedup_seed_test.go:23` | Verificar se marcador de dedup ainda faz sentido no novo contexto — se não, reescrever para afirmar curadoria |
| `server/finding_workflow_test.go:265` | `TestFindingWorkflowReporterCurates` — substituir o test de binding por verificação de que reporter popula `exfil_artifacts` |

### Critério de aceite
```bash
go test ./agent/... -run TestPromptSeedsCarryKSIRCurationMarker -v
# esperado: PASS
# Teste funcional: criar finding → reporter disparado → exfil_artifacts populado
go test ./server/... -run TestFindingWorkflowReporterCurates -v
```

### Rollback
Reverter `agent/promptcatalog.go`. A função de reseed preserva o histórico de versões em `agent_prompts` — reverter via UI de prompt do sistema.

---

## MÓDULO #6 — PolyEngine

### Objetivo
Sub-agente que intercepta resultado de tool rejeitado pelo alvo e replica o ataque com abordagem diferente: payload reescrito, ferramenta alternativa, vetor diferente, ou ordem de operação modificada. **Hook em `agent/toolcatalog.go` / `AugmentTools`**, não em `worker.go` após erro Go.

### Dependências
#2 (rate limiter removido — PolyEngine consome LLM por tentativa), #3 (adversarial gate removido — sem ele as variantes seriam descartadas).

### Arquivos

| AÇÃO | Caminho | O que muda |
|---|---|---|
| CRIAR | `agent/polyengine.go` | Struct `PolyEngine`, `Mutate(ctx, attempt PolyAttempt) (PolyAttempt, error)`, history, templates |
| CRIAR | `agent/errtoken.go` | `ParseRejection(result actool.Result) RejectionType` — classifica WAF/App/Network/Scope |
| CRIAR | `agent/polyengine_test.go` | Testes de mutação por tipo de rejeição |
| MODIFICAR | `agent/toolcatalog.go` | `AugmentTools(tools []actool.CoreTool) []actool.CoreTool` — envolver tools ofensivas com wrapper que chama PolyEngine em rejeição |
| CRIAR | `agent/payloads/php.go` | Templates PHP: webshell, reverse shell, filter chain, phar |
| CRIAR | `agent/payloads/powershell.go` | Templates PS: encoded, AMSI bypass, reflection |
| CRIAR | `agent/payloads/java.go` | Templates Java: EL injection, deserialization gadgets |

### Símbolos
- `agent/tools.go:1362, 1370, 1381, 1389` — exemplos de `actool.Errorf(...), nil` — erro dentro de `Result`, não como `error` Go
- `agent/toolcatalog.go` — local do hook via `AugmentTools`

### Contratos

```go
// agent/polyengine.go

type RejectionType string
const (
    RejectionWAF      RejectionType = "waf"
    RejectionSyntax   RejectionType = "syntax"
    RejectionAuth     RejectionType = "auth"
    RejectionTimeout  RejectionType = "timeout"
    RejectionScope    RejectionType = "scope"  // não tentar de novo
    RejectionUnknown  RejectionType = "unknown"
)

type PolyAttempt struct {
    ToolName    string
    Args        map[string]any
    RawResult   string
    Rejection   RejectionType
    MutationGen int
}

type PolyEngine struct {
    Provider llm.Provider
    Model    string
    MaxGens  int // máximo de gerações antes de desistir
}

// Mutate recebe uma tentativa rejeitada e retorna nova abordagem.
// Camada 1 (determinística): mutadores WAFFLED mecânicos (encoding, reordering, deadcode).
// Camada 2 (LLM): somente se camada 1 esgotar sem sucesso.
func (p *PolyEngine) Mutate(ctx context.Context, attempt PolyAttempt) (PolyAttempt, error)

// ParseRejection lê o Result de uma tool call e classifica o tipo de rejeição.
func ParseRejection(result actool.Result) RejectionType
```

**Mutadores WAFFLED (camada determinística, em ordem):**
1. W — Whitespace variation: inserir espaços/tabs/newlines entre tokens
2. A — Alternate encoding: hex/unicode/url-encode para keywords bloqueadas
3. F — Fragment substitution: partir keywords em concatenação de string
4. F2 — Function wrapping: `eval(base64_decode(...))` em vez de literal
5. L — Line comments injection: `/*x*/` entre tokens SQL/PHP
6. E — Encoding layer: duplo encoding onde aceito
7. D — Deadcode injection: adicionar ramos mortos para mudar hash de assinatura

### Testes que quebram
Nenhum existente quebra. Adicionar `agent/polyengine_test.go` com testes unitários.

### Critério de aceite
```bash
go test ./agent/... -run TestPolyEngine -v
# TestPolyEngineMutatesWAFRejection: dado resultado com corpo "WAF blocked", Mutate retorna PolyAttempt com encoding diferente
# TestPolyEngineStopsOnScope: dado RejectionScope, Mutate retorna erro (não tenta novamente)
# TestPolyEngineDeterministicFirst: camada LLM não é chamada se mutador mecânico suficiente
go build -tags embedui -o artex ./cmd/artex
```

### Rollback
Deletar `agent/polyengine.go`, `agent/errtoken.go`, `agent/payloads/`. Reverter `agent/toolcatalog.go`. Sem impacto em dados.

---

## MÓDULO #5 — PHP2Shell Cluster

### Objetivo
Transformar o skill php2shell (atualmente só `SKILL.md`) em cluster de exploração em massa. PoCs criados do zero (B7). Gates removidos.

### Dependências
#3 (worker sem gates), #6 (PolyEngine para payloads polimórficos).

### Arquivos

| AÇÃO | Caminho | O que muda |
|---|---|---|
| REESCREVER | `skills/php2shell/SKILL.md` | Remover gates (Authorization, Lab first, One-shot verification). Libertar chains 2/3. Adicionar modo cluster. |
| CRIAR | `skills/php2shell/cluster/scanner.py` | Recebe lista de URLs com LFI confirmado, testa em paralelo (50 threads) |
| CRIAR | `skills/php2shell/cluster/exploit.py` | Tenta chain 1 contra cada alvo, coleta output |
| CRIAR | `skills/php2shell/cluster/verifier.py` | Confirma RCE com comando marker |
| CRIAR | `skills/php2shell/cluster/payload_gen.py` | Gera variações polimórficas (WAFFLED) do payload PHP |
| CRIAR | `skills/php2shell/cluster/report.py` | Consolida: hosts explorados, shells obtidas, output |
| CRIAR | `skills/php2shell/payloads/ws_basic.php` | Webshell básico |
| CRIAR | `skills/php2shell/payloads/ws_obf_b64.php` | Webshell base64 |
| CRIAR | `skills/php2shell/payloads/ws_filter_chain.php` | PHP filter chain (PHP 8.x) |
| CRIAR | `skills/php2shell/payloads/ws_phar.php` | Phar desserialização |
| CRIAR | `skills/php2shell/payloads/rs_proc_open.php` | Reverse shell via proc_open |
| CRIAR | `skills/php2shell/poc/verify_session_injection.py` | Chain 1 end-to-end (criado do zero) |
| CRIAR | `skills/php2shell/poc/verify_handler_mismatch.py` | Chain 3 lab (criado do zero) |
| CRIAR | `skills/php2shell/poc/verify_uaf_asan.php` | Chain 2 ASAN (criado do zero) |
| CRIAR | `skills/php2shell/lab/setup.sh` | Docker lab vulnerável (criado do zero) |

**Nota B6:** CVE-2026-109618 e CVE-2026-109619 referenciados no `SKILL.md` existente não foram verificados em fonte primária. Manter no SKILL.md com nota `[PROVISIONAL — não confirmado em fonte externa]`.

### Contratos
Scripts Python — sem contratos Go. Interface: stdin/stdout ou argumentos CLI.

```python
# cluster/scanner.py
# Uso: scanner.py --targets urls.txt --threads 50 --timeout 10
# Output: JSON {url, php_version, vulnerable: bool, chain: "1|2|3|none"}

# cluster/exploit.py
# Uso: exploit.py --targets scan_results.json --chain 1 --threads 20
# Output: JSON {url, status: "shell_obtained|failed", shell_path, output}
```

### Testes que quebram
Nenhum existente. Criar `skills/php2shell/test_cluster.sh` que sobe o lab e roda o cluster contra ele.

### Critério de aceite
```bash
# Lab sobe
bash skills/php2shell/lab/setup.sh
# Scanner detecta versão vulnerável
python skills/php2shell/cluster/scanner.py --targets test_urls.txt
# Chain 1 obtém shell
python skills/php2shell/cluster/exploit.py --targets scan_out.json --chain 1
# Verifier confirma RCE
python skills/php2shell/cluster/verifier.py --shells shells.json
```

### Rollback
Deletar os arquivos CRIAR. Reverter `SKILL.md`. Sem impacto em DB.

---

## MÓDULO #4 — DB + Exploits Recentes

### Objetivo
Popular o banco com tabelas de payloads/chains/exfil e seed de CVEs corrigido. Skills novas de exploit.

### Dependências
#3 (schema de findings atualizado com `exploit_chain`/`exfil_path`).

### Arquivos

| AÇÃO | Caminho | O que muda |
|---|---|---|
| MODIFICAR | `db/schema.sql` | Adicionar tabelas `exploit_payloads`, `exploit_chains`, `exfil_artifacts` (ver DDL abaixo) |
| CRIAR | `db/seed_exploits.sql` | CVEs corrigidos conforme tabela §1.11. 10 skills (não 20). |
| CRIAR | `db/exfil_artifacts.go` | CRUD para `exfil_artifacts` |
| CRIAR | `db/exploit_payloads.go` | CRUD para `exploit_payloads` |
| CRIAR | `skills/rce-direct/SKILL.md` | Payloads RCE diretos por linguagem |
| CRIAR | `skills/sqli-to-rce/SKILL.md` | Cadeias SQLi→RCE (xp_cmdshell, UDF, LOAD DATA, COPY) |
| CRIAR | `skills/file-upload-weaponize/SKILL.md` | Upload → webshell → RCE |
| CRIAR | `skills/deserialize-chain/SKILL.md` | Desserialização → RCE (Java/.NET/PHP/Python) |
| CRIAR | `skills/ssrf-to-rce/SKILL.md` | SSRF → IMDS → credenciais cloud → RCE |
| MODIFICAR | `db/exploration.go` | Suporte a nós `exfil_artifact` no grafo |

### Símbolos
- `db/schema.sql:1485` linhas (confirmar antes de adicionar)
- `db/task_credentials.go` — já existe, não criar paralelo (reconciliação §1.9 da auditoria)
- `agent/tools_creds.go:17` — `saveTaskCredential()` já existe
- `agent/tools_creds.go:63` — `listTaskCredentials()` já existe

**Reconciliação de schema:** `exfil_artifacts` é para artefatos de exfiltração geral (arquivo, dump, screenshot). `task_credentials` é para credenciais extraídas. Uma credencial pode ter entrada em `task_credentials` **e** entrada em `exfil_artifacts` com `artifact_type = 'credential'` + `credential_ref → task_credentials.id`. Não duplicar — o registro em `task_credentials` é autoritativo para uso em lateral movement (#9).

### DDL completo

```sql
-- Payloads
CREATE TABLE IF NOT EXISTS exploit_payloads (
    id BIGSERIAL PRIMARY KEY,
    name TEXT NOT NULL,
    cve TEXT,
    target_type TEXT,        -- php/aspx/jsp/java/windows/linux/network
    target_version TEXT,
    payload_type TEXT,       -- reverse_shell/bind_shell/webshell/loader/sqli
    payload_body TEXT NOT NULL,
    compiled_binary BYTEA,
    obfuscation_template TEXT,
    requirements TEXT[],
    reliability TEXT CHECK (reliability IN ('high','medium','low')),
    confidence TEXT DEFAULT 'confirmed', -- 'confirmed' | 'provisional'
    created_at TIMESTAMPTZ DEFAULT now()
);

-- Cadeias de exploit
CREATE TABLE IF NOT EXISTS exploit_chains (
    id BIGSERIAL PRIMARY KEY,
    name TEXT NOT NULL,
    description TEXT,
    steps JSONB NOT NULL,
    trigger_condition TEXT,
    created_at TIMESTAMPTZ DEFAULT now()
);

-- Artefatos exfiltrados
CREATE TABLE IF NOT EXISTS exfil_artifacts (
    id BIGSERIAL PRIMARY KEY,
    task_id BIGINT REFERENCES tasks(id) ON DELETE CASCADE,
    finding_id BIGINT REFERENCES findings(id) ON DELETE SET NULL,
    artifact_type TEXT CHECK (artifact_type IN ('credential','dump','screenshot','source','config','file','token')),
    file_path TEXT,
    file_size BIGINT,
    hash_sha256 TEXT,
    credential_ref BIGINT REFERENCES task_credentials(id) ON DELETE SET NULL,
    compressed BOOLEAN DEFAULT false,
    encrypted BOOLEAN DEFAULT false,
    stored_path TEXT,
    value_rank INT DEFAULT 0,  -- score para RankArtifacts (#1)
    created_at TIMESTAMPTZ DEFAULT now()
);
```

**CVE seed corrigido** (conforme §1.11 da auditoria):

| CVE | Linguagem/Plataforma | Status |
|---|---|---|
| CVE-2024-4577 | PHP | manter |
| CVE-2023-0669 | **Java** (GoAnywhere MFT) | movido de PHP para Java |
| CVE-2025-24813 | Java (Apache Tomcat) | manter |
| CVE-2024-38475 | **Web server** (httpd mod_rewrite) | movido de Java para web-server |
| CVE-2024-38077 | Windows | manter |
| CVE-2024-49112 | Windows | manter |
| CVE-2024-21338 | Windows | manter |
| CVE-2024-1086 | Linux | manter |
| CVE-2024-53104 | Linux | reclassificar: out-of-bounds read, não RCE/priv-esc |
| CVE-2024-38063 | Network | manter |
| CVE-2024-28995 | Network | manter |
| Log4Shell / CVE-2021-44228 | Java legado | manter somente se alvo legado. Seed com nota `target_version = 'legacy'` |
| Dirty COW / CVE-2016-5195 | Linux legado | idem |
| CVE-2026-109618/109619 | PHP | seed com `confidence = 'provisional'` |

### Testes que quebram
Nenhum existente quebra. Novo: `db/exfil_artifacts_test.go` com CRUD básico.

### Critério de aceite
```bash
go test ./db/... -run TestExfilArtifacts -v
# Tabelas criadas
psql -c "\d exfil_artifacts"
# Seed de exploits populado
psql -c "SELECT count(*) FROM exploit_payloads"
# esperado: >10
```

### Rollback
DDL usa `IF NOT EXISTS` — não quebra DB existente. Reverter `schema.sql`. Apagar tabelas manualmente se necessário.

---

## MÓDULO #1 — Exfiltração (substitui Relatórios)

### Objetivo
Transformar o endpoint de relatório em sistema de exfiltração. Coletar, classificar, empacotar e entregar artefatos ao operador. Dependência positiva em `workerArtifactSpec` (seção [C] do prompt, code-owned) — os artefatos aterrisam no run dir da tarefa.

### Dependências
#3b (reporter cura e popula `exfil_artifacts`), #4 (`exfil_artifacts` schema existe).

### Arquivos

| AÇÃO | Caminho | O que muda |
|---|---|---|
| CRIAR | `exfil/exfil.go` | Package principal: `CollectFiles`, `CollectCredentials`, `CollectDBDumps`, `RankArtifacts`, `PackageArchive`, `StreamToOperator` |
| CRIAR | `exfil/collector.go` | Varredura do workdir por arquivos extraídos |
| CRIAR | `exfil/archive.go` | Empacotamento em `.tar.gz` |
| CRIAR | `exfil/ranker.go` | `RankArtifacts(taskID)` — score por tipo (credential=100, config=80, token=90, dump=60, source=40, screenshot=20) |
| MODIFICAR | `server/server.go:817` | Renomear handler `getReport` → `getExfil`. Rota: `GET /api/exfil?task=X&format=json|archive`. |
| MODIFICAR | `web/src/lib/api.ts:873` | `api.report(task)` → `api.exfil(task, format)` |
| SUBSTITUIR | `web/src/app/(main)/function/tasks/detail/_tabs/report-tab.tsx` | → `exfil-tab.tsx` — árvore de arquivos, filtro por tipo, botões Download/Copy/View |
| MODIFICAR | `web/src/app/(main)/function/tasks/detail/page.tsx` | Trocar import/uso da aba |
| MODIFICAR | `agent/toolcatalog.go` | Registrar tool `exfil_data` |
| MODIFICAR | `agent/tools_insert.go:656` | `WorkerTools()` — adicionar `exfil_data` |
| CRIAR | `agent/tools_exfil.go` | Tool `exfil_data(target_path, pattern, max_size_mb)` |

**Nota: `report/report.go` e `report/findings.go` NÃO são deletados** — são usados pelo handler existente e podem ser necessários para compatibilidade. Handler `getReport` em `server/server.go:817` é renomeado, não removido.

**`RankArtifacts` (ponta solta #12):** Classificar antes de empacotar. Dump de 40GB vale menos que 200 linhas de `web.config`. Score define ordem no manifesto e prioridade de download.

### Contratos

```go
// exfil/exfil.go

type Artifact struct {
    ID         int64
    TaskID     int64
    Type       string // credential/dump/screenshot/source/config/file/token
    Path       string
    SizeMB     float64
    SHA256     string
    ValueRank  int
    StoredPath string
}

// CollectFiles varre o workdir da tarefa por arquivos extraídos.
func CollectFiles(ctx context.Context, db *db.DB, taskID int64, pattern string) ([]Artifact, error)

// RankArtifacts classifica artifacts por valor (score decrescente).
func RankArtifacts(artifacts []Artifact) []Artifact

// PackageArchive empacota artifacts em .tar.gz no diretório operacional.
func PackageArchive(ctx context.Context, taskID int64, artifacts []Artifact) (archivePath string, err error)

// CollectCredentials agrega credenciais de task_credentials pelo taskID.
func CollectCredentials(ctx context.Context, db *db.DB, taskID int64) ([]Artifact, error)

// agent/tools_exfil.go — tool exfil_data
type ExfilDataArgs struct {
    TargetPath string `json:"target_path"`
    Pattern    string `json:"pattern"`
    MaxSizeMB  int    `json:"max_size_mb"`
}
```

### Testes que quebram
Nenhum existente de exfiltração (funcionalidade nova). Verificar se remoção de `getReport` quebra algum teste em `server/`.

```bash
grep -r "getReport\|/api/report" server/*_test.go
```

### Critério de aceite
```bash
go test ./exfil/... -v
# TestRankArtifactsCredentialFirst: credencial tem score > dump de arquivo
# TestPackageArchive: .tar.gz gerado contém todos os artifacts
go test ./server/... -run TestExfilEndpoint -v
# GET /api/exfil?task=1&format=json retorna manifesto com artifacts rankeados
cd web && npm run build:static
# Frontend compila sem erro
```

### Rollback
Reverter `server/server.go` e `web/src/lib/api.ts`. Deletar `exfil/`. Restaurar `report-tab.tsx`. O endpoint `/api/report` volta se o handler for revertido.

---

## MÓDULO #8 — Grafo de Proxy Dinâmico

### Objetivo
Infraestrutura de proxy com rotação por tool call, encadeada obrigatoriamente **através do MITM de egresso existente** (`agent/worker.go:206-214`). Nunca em paralelo — quebraria `traffic_*` e a cadeia de evidência dos módulos #1 e #3b.

### Dependências
#1 (exfiltração funcionando — proxy deve preservar proveniência), #3b (reporter precisa de `traffic_*` para curar artefatos).

### Arquivos

| AÇÃO | Caminho | O que muda |
|---|---|---|
| CRIAR | `proxygraph/graph.go` | Grafo de proxies: nodes + edges + políticas |
| CRIAR | `proxygraph/pool.go` | Pool ativo com health check (latência, ASN diversity, score) |
| CRIAR | `proxygraph/rotator.go` | Rotação por tool call: IP, User-Agent, headers, TLS fingerprint |
| CRIAR | `proxygraph/traffic.go` | Modelagem de tráfego (não é responsabilidade do perfil C2 — ponta solta #6) |
| CRIAR | `proxygraph/providers/residential.go` | Pool de proxies residenciais via API |
| CRIAR | `proxygraph/providers/tor.go` | TOR circuit rotation |
| CRIAR | `proxygraph/exfil.go` | Fragmentação de dados em campos de requisição legítima |
| MODIFICAR | `agent/worker.go:215` | `proxyEnv()` → `proxyEnvRotated()` que consulta `proxygraph.Rotator`. A função atual (`proxyEnv`) **permanece** como fallback. |
| MODIFICAR | `agent/proxyenv_test.go` | Atualizar para novo comportamento |
| CRIAR | `web/src/app/(main)/system/proxy-graph/page.tsx` | Configuração + health status |
| CRIAR | `server/proxy_handler.go` | API endpoints `/api/proxygraph/*` |

**Arquivos que NÃO existem e NÃO devem ser citados:**
- `agent/proxyenv.go` — **não existe**. A função `proxyEnv` está em `agent/worker.go:215`.
- `server/proxy.go` — **não existe**. Criar `server/proxy_handler.go` como arquivo novo.

**Domain fronting (ponta solta #8):** AWS e Google desabilitaram. Microsoft bloqueou para `login.microsoftonline.com`. Substituir por serverless próprio: Cloudflare Workers com domínio próprio configurado, tráfego mimético (não fronting real). Remover referência a domain fronting do `proxygraph/traffic.go`.

**TLS fingerprint:** só se aplica ao cliente HTTP Go do ARTEX — não ao tráfego de subcomandos Bash (curl/nuclei/etc.). Para subcomandos, rotação de fingerprint deve acontecer no proxy de egresso.

### Contratos

```go
// proxygraph/pool.go
type Proxy struct {
    ID      string
    URL     string
    Type    string // residential/tor/datacenter
    ASN     string
    Score   float64
    Active  bool
}

// Rotator seleciona próximo proxy da pool por política.
type Rotator interface {
    Next(ctx context.Context, policy RotationPolicy) (Proxy, error)
}

type RotationPolicy string
const (
    PolicyRoundRobin  RotationPolicy = "round_robin"
    PolicyASNDiverse  RotationPolicy = "asn_diverse"
    PolicyLeastUsed   RotationPolicy = "least_used"
)

// proxygraph/traffic.go
// MimeticProfile gera headers/UA que imitam tráfego legítimo.
type MimeticProfile struct {
    Domain     string // domínio alvo de imitação
    UserAgent  string
    Headers    map[string]string
}

func NewMimeticProfile(style string) MimeticProfile // "m365", "google", "aws", "generic"
```

**Teste de integração de evidência (ponta solta #11):**
```go
// server/proxygraph_integration_test.go
// TestProxyRotationPreservesTrafficCapture:
//   1. Criar tarefa com captura ligada
//   2. Tool call via proxy rotativo
//   3. Afirmar que traffic_search retorna o registro com proxy_hop preenchido
```

### Testes que quebram

| Arquivo | Correção |
|---|---|
| `agent/proxyenv_test.go` | Atualizar: testar `proxyEnv("")` retorna nil (comportamento original mantido como fallback), e `proxyEnvRotated()` com pool não-vazio retorna proxy da pool |

### Critério de aceite
```bash
go test ./proxygraph/... -v
# TestPoolHealthCheck: proxy inativo é removido da pool
# TestRotatorASNDiverse: dois proxies consecutivos têm ASNs diferentes
go test ./server/... -run TestProxyRotationPreservesTrafficCapture -v
# Tráfego via proxy rotativo aparece em traffic_search
CGO_ENABLED=0 go build -tags embedui -o artex ./cmd/artex
```

### Rollback
Deletar `proxygraph/`, `server/proxy_handler.go`. Reverter `agent/worker.go:215` para `proxyEnv` simples. Deletar página frontend.

---

## MÓDULO #7 — Delivery Assist C2

### Objetivo
Entregar o binário do RAT Windows na shell já obtida pelo worker e registrar o evento no grafo. **RAT sem API** (B1, B2, B3) — não existe automação de sessão. O ARTEX faz: detecção de oportunidade, staging do binário, entrega via shell existente, registra nó `c2_delivery` e passa controle ao operador via `ask_operator()`.

### Dependências
#8 (proxy disponível para staging seguro do binário), #9 (lateral movement usa a sessão entregue manualmente).

### Arquivos

| AÇÃO | Caminho | O que muda |
|---|---|---|
| CRIAR | `c2bridge/bridge.go` | Interface `C2Backend` — adaptada para RAT sem API |
| CRIAR | `c2bridge/delivery.go` | Vetores de entrega por tipo de shell: webshell upload, echo+base64, certutil, bitsadmin, wget, curl, SMB staging |
| CRIAR | `c2bridge/stage.go` | `Stage(arch string)` — verifica presença do binário RAT no workdir do operador, retorna path local |
| CRIAR | `c2bridge/config.go` | Configuração: path local do binário RAT por arch (x64/x86), staging dir |
| CRIAR | `agent/tools_c2.go` | Tools: `c2_stage_implant`, `c2_deliver_implant`, `c2_await_operator` |
| MODIFICAR | `agent/toolcatalog.go` | Registrar tools do c2bridge |
| MODIFICAR | `agent/tools_insert.go:656` | `WorkerTools()` — adicionar tools c2 |
| MODIFICAR | `db/schema.sql` | Adicionar tabela `c2_deliveries` (não `c2_sessions` — RAT não reporta sessão de volta) |
| CRIAR | `web/src/app/(main)/system/c2-bridge/page.tsx` | Configuração: path do binário RAT por arch, staging dir |
| CRIAR | `skills/c2-handoff/SKILL.md` | Doutrina de delivery por tipo de shell e de operação pós-handoff |

### Símbolos
Não há símbolos existentes a modificar além de `toolcatalog.go` e `tools_insert.go`.

### Contratos

```go
// c2bridge/bridge.go

// C2Backend interface — versão RAT sem API.
// Sem RequestImplant, ListListeners, Exec, Cleanup (RAT não expõe).
type C2Backend interface {
    // Stage verifica se o binário RAT está disponível localmente para o arch dado.
    Stage(ctx context.Context, arch string) (localPath string, err error)
    // Deliver envia o binário via o vetor e shell especificados.
    Deliver(ctx context.Context, vector DeliveryVector, binaryPath string) error
}

type DeliveryVector struct {
    Type       string // "webshell_upload", "echo_b64", "certutil", "bitsadmin", "wget", "curl"
    ShellURL   string // URL da webshell existente, se webshell_upload
    TargetPath string // path de destino no alvo
}

// c2bridge/delivery.go
func SelectDeliveryVector(shellType string) DeliveryVector
// shellType: "webshell_php", "rce_cmd", "sqli_exec", "ssh"

// agent/tools_c2.go

type StageImplantArgs struct {
    Arch string `json:"arch"` // "x64" | "x86"
}

type DeliverImplantArgs struct {
    Vector     string `json:"vector"`     // "webshell_upload" | "echo_b64" | ...
    ShellURL   string `json:"shell_url"`
    TargetPath string `json:"target_path"`
    Arch       string `json:"arch"`
}
```

**DDL tabela de entrega:**
```sql
CREATE TABLE IF NOT EXISTS c2_deliveries (
    id BIGSERIAL PRIMARY KEY,
    task_id BIGINT REFERENCES tasks(id) ON DELETE CASCADE,
    target_host TEXT NOT NULL,
    target_path TEXT,
    arch TEXT,
    delivery_vector TEXT,
    status TEXT CHECK (status IN ('delivered','failed','pending_operator')),
    delivered_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT now()
);
```

**Impacto do RAT sem API (declarado):** Após `c2_deliver_implant`, o worker chama `ask_operator()` com mensagem: "Implant entregue em TARGET_HOST via VECTOR. Verifique conexão de entrada no RAT e continue a operação manualmente." A autonomia de pós-exploração (módulo #9) depende de o operador confirmar a sessão e reiniciar o worker com intent de lateral movement.

### Testes que quebram
Nenhum existente. Criar `c2bridge/stage_test.go` com mock.

### Critério de aceite
```bash
go test ./c2bridge/... -v
# TestStageImplantFoundX64: binário x64 presente no staging dir retorna path correto
# TestDeliverViaEchoB64: generate echo+certutil command correto para target_path
# TestSelectDeliveryVector: webshell_php → webshell_upload, rce_cmd → echo_b64
CGO_ENABLED=0 go build -tags embedui -o artex ./cmd/artex
```

### Rollback
Deletar `c2bridge/`, `agent/tools_c2.go`, `web/.../c2-bridge/`. Reverter `toolcatalog.go` e `tools_insert.go`. Deletar tabela `c2_deliveries`.

---

## MÓDULO #9 — Movimentação Lateral + Priv-Esc

### Objetivo
Módulo agêntico de pós-exploração: enumeração → coleta de credenciais → priv-esc → movimento lateral → dominação de domínio. Roda via shell entregue pelo módulo #7. Cleanup via shell direta (B3 — RAT sem primitiva de cleanup).

### Dependências
#7 (shell/delivery no alvo), #4 (tabela `task_credentials` e `exfil_artifacts` existentes).

### Arquivos

| AÇÃO | Caminho | O que muda |
|---|---|---|
| CRIAR | `skills/lateral-movement/SKILL.md` | 5 fases: enumeração, credenciais, lateral, priv-esc local, dominação |
| CRIAR | `skills/lateral-movement/payloads/amsi_bypass.ps1` | AMSI bypass + PS ofuscado |
| CRIAR | `skills/lateral-movement/payloads/invoke_mimikatz.ps1` | Reflective load customizado |
| CRIAR | `agent/tools_lateral.go` | Tools: `lateral_scan`, `harvest_credentials`, `move_lateral`, `escalate_privilege`, `domain_escalate`, `check_edr`, `cleanup_artifacts` |
| MODIFICAR | `agent/toolcatalog.go` | Registrar tools laterais |
| MODIFICAR | `agent/tools_insert.go:656` | `WorkerTools()` — adicionar tools laterais |

**Reconciliação de credenciais (§1.9 da auditoria):**
- `dump_credentials` do v1 **não é criada** — duplicaria `saveTaskCredential()` em `agent/tools_creds.go:17`.
- Nova tool: `harvest_credentials(method)` — chama `saveTaskCredential()` internamente para cada credencial coletada.
- `listTaskCredentials()` em `agent/tools_creds.go:63` já provê listagem — não duplicar.

**Cleanup (B3 — sem primitiva no RAT):**
- Tool `cleanup_artifacts(target_host, paths []string)` implementa limpeza via comandos shell diretos:
  - Windows: `del /f /q PATH`, `reg delete KEY`, `sc delete SERVICE`
  - Linux: `rm -f PATH`, `crontab -r`
- Mais lento e mais ruidoso que primitiva de C2 nativa. Impacto declarado.

**`check_edr()` e catálogo de evasão (B2):**
Sem perfil negociável no RAT, `check_edr()` serve para **informar o operador**, não para parametrizar o implant. A tool retorna o EDR detectado e o operador decide manualmente se troca o binário do RAT por versão mais evasiva. Catálogo de referência (§2.9 da auditoria) vai em `skills/lateral-movement/SKILL.md` como seção de decisão.

### Contratos

```go
// agent/tools_lateral.go

type LateralScanArgs struct {
    Target string `json:"target"`
}

type HarvestCredentialsArgs struct {
    Method string `json:"method"` // lsass/sam/ntds/dpapi/kerberos/files/history
    Target string `json:"target"`
}

type MoveLateralArgs struct {
    Target       string `json:"target"`
    Method       string `json:"method"` // wmi/winrm/smb/ssh/psexec/dcom/rdp
    CredentialID int64  `json:"credential_id"` // ref task_credentials.id
}

type EscalatePrivilegeArgs struct {
    Method string `json:"method"` // token_steal/service_hijack/uac_bypass/potato/kernel
    Target string `json:"target"`
}

type DomainEscalateArgs struct {
    Method string `json:"method"` // dcsync/golden_ticket/kerberoast/asrep_roast/gpo
}

type CheckEDRArgs struct {
    Target string `json:"target"`
}

type CleanupArtifactsArgs struct {
    Target string   `json:"target"`
    Paths  []string `json:"paths"`
    OS     string   `json:"os"` // windows/linux
}
```

**Nó `identity` no asset graph (ponta solta #13):**
```sql
-- Adicionar ao schema.sql — módulo #9 cria nós de identidade
CREATE TABLE IF NOT EXISTS identity_nodes (
    id BIGSERIAL PRIMARY KEY,
    task_id BIGINT REFERENCES tasks(id) ON DELETE CASCADE,
    email TEXT,
    username TEXT,
    source TEXT,              -- credential_harvest/osint/phishing
    credential_ref BIGINT REFERENCES task_credentials(id) ON DELETE SET NULL,
    tenant_domain TEXT,
    created_at TIMESTAMPTZ DEFAULT now()
);
```

### Testes que quebram
Nenhum existente direto. Criar `agent/tools_lateral_test.go` com mocks.

### Critério de aceite
```bash
go test ./agent/... -run TestLateralTools -v
# TestHarvestCredentialsCallsSaveCred: harvest_credentials chama saveTaskCredential
# TestCleanupArtifactsWindowsCmd: gera comando del /f /q correto
# TestCheckEDRReturnsName: retorna nome do EDR detectado
CGO_ENABLED=0 go build -tags embedui -o artex ./cmd/artex
```

### Rollback
Deletar `agent/tools_lateral.go`, `skills/lateral-movement/`. Reverter `toolcatalog.go` e `tools_insert.go`. Reverter alterações de schema (tabelas `IF NOT EXISTS` são non-breaking).

---

## MÓDULO #10 — Superfície de Nuvem e Identidade

### Objetivo
Atacar a nuvem do alvo: cadeia de confiança OIDC, identidades gerenciadas Azure/AWS, IMDS, estado Terraform. Pipeline de OSINT de identidade: logs de infostealer → credential stuffing → device code OAuth. Paralelo com #9.

### Dependências
#4 (`exfil_artifacts` e `exploit_payloads` existentes), #9 (nó `identity` criado no schema).

### Arquivos

| AÇÃO | Caminho | O que muda |
|---|---|---|
| CRIAR | `skills/oidc-trust-abuse/SKILL.md` | Cadeia OIDC → takeover de IAM role AWS. "Sub:jugation" — reciclagem de namespace. |
| CRIAR | `skills/azure-identity-chain/SKILL.md` | Identidade gerenciada → IMDS Azure → RBAC → Key Vault dump. |
| CRIAR | `skills/cicd-injection/SKILL.md` | GitHub Actions runner self-hosted. Camadas Lambda. `pull_request_target` bypass. |
| CRIAR | `skills/osint-identity/SKILL.md` | Logs de infostealer → stuffing → device code OAuth 2.0 (EvilTokens flow). AiTM Evilginx. |
| CRIAR | `agent/tools_cloud.go` | Tools: `enumerate_oidc_chain`, `query_imds`, `extract_tf_state`, `device_code_init`, `check_mfa_methods` |
| MODIFICAR | `agent/toolcatalog.go` | Registrar tools de nuvem |
| MODIFICAR | `agent/tools_insert.go:656` | `WorkerTools()` |

**Nota sobre device code OAuth (§2.2 item 15 da auditoria):** A técnica abusa do *device authorization grant* legítimo. A tool `device_code_init` inicia o fluxo e retorna o user_code para o operador usar em campanha de engenharia social. Sem automação de phishing — o canal de entrega do código é responsabilidade do operador.

**Nota sobre WebAuthn (§2.2 item 17):** FIDO2/WebAuthn é resistente a phishing. A tool `check_mfa_methods` enumera métodos de fallback do tenant (SMS, push, voz) antes de tentar contorno. Se só WebAuthn estiver disponível, a tool retorna `mfa_bypass_possible: false`.

### Contratos

```go
// agent/tools_cloud.go

type EnumerateOIDCChainArgs struct {
    Domain string `json:"domain"` // domínio da organização alvo
}

type QueryIMDSArgs struct {
    Target   string `json:"target"` // IP do host comprometido
    Version  string `json:"version"` // "v1" | "v2"
    Provider string `json:"provider"` // "aws" | "azure" | "gcp"
}

type ExtractTFStateArgs struct {
    BucketURL string `json:"bucket_url"` // S3/GCS/Azure Blob URL do state
}

type DeviceCodeInitArgs struct {
    TenantDomain string `json:"tenant_domain"`
    ClientID     string `json:"client_id"` // app registration legítimo
    Scope        string `json:"scope"`     // "offline_access openid profile"
}

type CheckMFAMethodsArgs struct {
    TenantDomain string `json:"tenant_domain"`
    Username     string `json:"username"`
}
```

### Testes que quebram
Nenhum existente. Criar `agent/tools_cloud_test.go` com mocks HTTP.

### Critério de aceite
```bash
go test ./agent/... -run TestCloudTools -v
# TestQueryIMDSv2RequiresPUT: IMDSv2 gera PUT de token antes do GET de credencial
# TestCheckMFAMethodsWebAuthnReturnsFalse: WebAuthn-only retorna mfa_bypass_possible=false
# TestExtractTFStateFindsCredentials: state JSON com senha em plaintext é detectado
CGO_ENABLED=0 go build -tags embedui -o artex ./cmd/artex
```

### Rollback
Deletar `skills/oidc-trust-abuse/`, `skills/azure-identity-chain/`, `skills/cicd-injection/`, `skills/osint-identity/`, `agent/tools_cloud.go`. Reverter `toolcatalog.go` e `tools_insert.go`.

---

## VERIFICAÇÃO POR FASE

Cada fase deve fechar com os três comandos verdes antes da próxima começar:

```bash
CGO_ENABLED=0 go build -tags embedui -o artex ./cmd/artex
go test ./...
cd web && npm run build:static
```

**Subconjunto de testes por fase:**

| Fase | Módulos | Testes afetados |
|---|---|---|
| 0 | #0, #0b | `./db/intercept*`, `./server/intercept*`, `./agent/*seed*`, `./agent/*gate*` |
| 1 | #2, #3, #3b, #6 | `./server/notify*`, `./agent/finding*`, `./agent/poly*` |
| 2 | #5, #4, #1 | `./db/exfil*`, `./exfil/...`, `./server/...` (endpoint exfil) |
| 3 | #8, #7, #9, #10 | `./proxygraph/...`, `./c2bridge/...`, `./agent/tools_lateral*`, `./agent/tools_cloud*` |

**Nota sobre seed tests na Fase 0:** `go test ./...` vai falhar nos seed tests até a reescrita de marcadores estar feita no módulo #0b. Isso é **esperado e documentado**. O executor não deve tratar como regressão — é o comportamento correto de especificação executável defendendo o novo estado.

**Teste funcional de ponta (por fase):**
Stack Docker sobe → tarefa de exploração criada → comportamento novo observado:
- Fase 0: worker executa tool sem ficar em `intercept_pending`
- Fase 1: worker usa payload polimórfico após rejeição
- Fase 2: finding confirmado → reporter popula `exfil_artifacts` → endpoint `/api/exfil` retorna manifesto
- Fase 3: proxy rotativo preserva `traffic_search`, `c2_deliver_implant` registra nó em grafo

---

## GUARDRAILS

- Nunca escrever em `main`/`master`. Worktree própria por ticket.
- Sem push, merge, deploy ou mudança de segredo sem autorização explícita.
- **AGPL-3.0**: ARTEX é fork BaaRai do Autumn-27/ARTEX. Modificações permanecem sob a mesma licença.
- `SKIP` e `PASS_WITH_WARNINGS` não contam como evidência de conclusão.
- `agent-scripts.sh` referenciado em `C:\Users\error\AGENTS.md` — verificar se existe antes de usar. Se não existir, usar os comandos da seção Verificação acima.
- **Escopo autorizado (B5) é inegociável.** Regras de intercept de escopo são preservadas. O módulo #0 desarma regras `deny`/`ask` ofensivas, não o sistema de escopo.
- **CGO_ENABLED=0** em todos os módulos Go. Zero dependência CGO. Zero Win32 API no código Go do ARTEX.

---

## ROADMAP FUTURO (não implementado no v2)

- **RAG vetorial entre workers e entre tarefas** (ponta solta #14): repositório de conhecimento compartilhado que persiste descobertas entre sessões. Estruturalmente diferente do ARTEX atual — exige embedding store (pgvector ou Qdrant). Decisão: não bloquear o v2. Registrar em `planos/ROADMAP-FUTURO.md`.
- **MQTT / QUIC / WebSocket como canal C2** (§2.3 itens 22-24): relevante quando o RAT for substituído por implant com canal configurável.
- **Dead drop resolver** (MITRE T1102): GitHub/Telegram como ponto de encontro para instrução C2.
- **Providers cloud efêmeros para proxygraph** (AWS EC2 spot, Azure ACI): infra cloud do operador como nó de proxy. Não implementado neste v2 por exigir credenciais cloud do operador e acoplamento de infra.
