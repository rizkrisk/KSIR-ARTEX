# Handoff — Consolidar o plano KSIR v2 (executor-ready)

**Data:** 2026-10-27
**Projeto:** `C:\Users\error\Desktop\ARTEX-main\ARTEX-main\`
**Tipo:** lite handoff (sessão anterior → agente revisor → agente executor)

---

## 1. Objetivo

Produzir **`planos/KSIR-WEAPONIZATION-v2.md`** — o plano de weaponização do ARTEX corrigido, reestruturado e sem erros factuais, pronto para ser entregue a um **agente executor** que vai implementar sem precisar interpretar nada.

A sessão anterior **auditou** o plano e documentou os defeitos. Ela **não corrigiu** o plano. Essa é a tua tarefa.

**Não re-auditar.** A auditoria está pronta e verificada contra o código. Consumir e aplicar.

---

## 2. Artefatos existentes (ler nesta ordem)

| Arquivo | Linhas | Papel |
|---|---|---|
| `planos/KSIR-AUDIT-COMPLEMENTO.md` | 854 | **Fonte de verdade das correções.** Auditoria + 88 complementos + prioridades + riscos |
| `planos/KSIR-WEAPONIZATION.md` | 685 | Plano original (v1). Base estrutural, mas contém os erros catalogados na auditoria |
| `planos/PROMPT-AUDITOR.md` | — | Brief que gerou a auditoria. Já consumido, não re-executar |

O v2 **substitui** o v1. Não editar o v1 em cima — escrever arquivo novo e manter o v1 como registro histórico.

---

## 3. O que a auditoria já estabeleceu (não rediscutir)

Tudo abaixo está **verificado contra o código real** na §1 do documento de auditoria. O v2 precisa incorporar, não reavaliar.

**Veredicto:** 0 dos 9 módulos do v1 é executável como escrito. 4 em FALHA bloqueante (#3, #5, #6, #7, #8), 5 INCOMPLETO.

**Três bloqueios estruturais** — detalhes em §1.2, §1.3, §1.4, §1.7:
1. Seed de prompt é *first-insert-only* → reescrever const Go não afeta DB existente. Exige migração de reseed.
2. Intercept (`intercept/intercept.go`, 690 linhas, ações `allow`/`deny`/`ask`) e constraints (`constraintBlock()` em `agent/worker.go:446` e `agent/planner.go:412`) são gates de **runtime** e o v1 não os toca.
3. Módulo #7 tem contradição de build insanável (`CGO_ENABLED=0` vs injector CGO/Win32) + erro arquitetural. Reescrito como **Handoff C2** (`c2bridge/`).

**Correções factuais obrigatórias** — tabela completa em §1.10: 5 arquivos fantasmas, 5 linhas/intervalos errados, contagem de skills errada (10, não 20).

**Módulos novos que o v1 não tem** — especificados na §2:
- **#0** Desarme de Intercept + Constraints (§1.3, §1.4)
- **#0b** Migração de reseed de prompts (§1.2)
- **#3b** Repurpose do agente `reporter` (§1.6)
- **#10** Superfície de Nuvem e Identidade (§2.4)
- Pipeline de OSINT passivo (§2.1) e OSINT de identidade (§2.2)
- `clean_artifacts()` como passo final da árvore do #9 (§2.10, item 80)

**Ordem de execução corrigida** (o v1 tem a ordem errada) — está no fim da §1.13:
```
Fase 0  #0 Intercept+Constraints → #0b Migração de reseed
Fase 1  #2 Rate limiters → #3+#3b Worker/Reporter → #6 PolyEngine
Fase 2  #5 PHP2Shell → #4 DB/CVE → #1 Exfiltração
Fase 3  #8 Proxy → #7 Handoff C2 → #9 Lateral
```
Justificativa das arestas em §1.13. Duas mudanças contra-intuitivas que precisam estar explicadas no v2: **#6 foi antecipado** da Fase 2 para a Fase 1 (depende de #2 e só faz sentido sem os gates), e **#7 vem antes de #9** (lateral roda através da sessão C2).

**Inventário de testes que quebram** — §1.12, 18+ arquivos. Decisão já tomada e que o v2 deve carregar: **reescrever os marcadores, não deletar os testes**. Os seed tests são especificação executável; sem eles não há garantia de que a migração de reseed funcionou.

**Reconciliações pendentes entre módulos** — §1.9 e §1.13: `exfil_artifacts` (#4) vs. `db/task_credentials.go` vs. tabela de credenciais (#9) apontam para o mesmo dado. `dump_credentials` é proposta duas vezes (#3 e #9). `proxygraph/traffic.go` (#8) sobrepõe malleable profile do C2 (§2.11, item 88).

---

## 4. Bloqueios — precisam de resposta do operador

Estes **não** se resolvem lendo código. Levantar com Maik antes de fechar o v2. Sem resposta, o v2 fica com lacuna e o executor vai inventar.

| # | Pergunta | Onde trava |
|---|---|---|
| B1 | Qual a superfície de API do painel C2? Precisa de: geração de implant por arch/OS, listagem de listeners, execução em sessão, upload/download. | Módulo #7 inteiro (§1.7). Sem API → handoff degrada para entrega manual → autonomia morre |
| B2 | O painel aceita **perfil de evasão negociável** na geração do implant? | Matriz `check_edr()` → perfil (§2.9, item 73). Se não, a matriz vira documentação inútil e `c2bridge/implant.go` não tem o que parametrizar |
| B3 | O painel expõe primitiva de **cleanup** pós-operação? | §2.10 item 80. Se não, o #9 implementa por conta via `c2_exec` — mais lento e mais ruidoso |
| B4 | Agente `reporter`: **Opção A** (repurpose para curadoria de exfiltração) ou **Opção B** (desligar via flag `finding_workflow_tools_v2_reporter`)? | §1.6. Opção B órfã `bind_finding_traffic` e mata a proveniência de artefato que o #1 precisa |
| B5 | Intercept: desarme total, ou **manter o limite de escopo autorizado** como trilho inegociável? | §1.3 e §2.8. A recomendação da auditoria é manter o trilho de escopo — remover tudo elimina a única proteção contra o ARTEX atingir infraestrutura fora do alvo |
| B6 | CVE-2026-109618 / CVE-2026-109619 existem? | §1.11. Vêm só do `skills/php2shell/SKILL.md`; não confirmadas em fonte externa. Module #4 semeia o DB com elas |
| B7 | O módulo #5 assume que os PoCs do php2shell serão **criados do zero**? | §1.1. O v1 descreve 4 scripts como "estado atual" e nenhum existe. Custo não orçado |

---

## 5. Pontas soltas técnicas

Resolver durante a consolidação. Nenhuma exige o operador.

1. **Especificação do hook do PolyEngine.** O v1 diz "hook no `worker.go` após cada tool call que retorna erro". Impossível: tools retornam `actool.Errorf(...), nil` — o erro vai dentro do `Result`, não como `error` Go (evidência em `agent/tools.go:1362, 1370, 1381, 1389`). Corrigir para wrap em `agent/toolcatalog.go` / `AugmentTools`. Detalhe em §1.5.
2. **Elevar o escopo do #6.** De "reescrever payload" para "replanejar abordagem" (payload, ferramenta, vetor, ordem). Justificativa e referência em §2.8 item 62.
3. **Camada determinística antes do LLM no #6.** Mutadores mecânicos (WAFFLED) primeiro, LLM só quando esgotar. Corta consumo de token em ordem de magnitude. §2.5 item 45.
4. **Registro de tools novas precisa de dois lugares**, não um: `agent/toolcatalog.go` **e** `agent/tools_insert.go:656/687/715` (`WorkerTools`/`MainAgentTools`/`AllDomainTools`). O v1 cita só o primeiro. §1.9.
5. **Interface `C2Backend`** desacoplada do painel. Assinatura pronta em §2.11 item 87 — usar como está.
6. **Módulo #8: separar responsabilidades.** `proxygraph` = rotação de saída (IP/ASN/TLS fingerprint/pool). Forma do tráfego = perfil do C2. §2.11 item 88.
7. **Módulo #8: encadear através do MITM de egresso existente**, nunca em paralelo. O MITM (`agent/worker.go:206-214`) alimenta `traffic_*` → `bind_finding_traffic` → evidência de finding. Rotear por fora quebra o #1 e o #3b. §1.8 item 4.
8. **Corrigir expectativa de domain fronting.** AWS e Google desabilitaram; Microsoft bloqueou para `login.microsoftonline.com`. Viável em 2026 é serverless próprio (Workers, MITRE T1583.007) com tráfego *parecido*. §2.3 item 27.
9. **CVEs do #4**: 5 mal atribuídas, 2 são de 2016/2021 contradizendo o framing "recentes". Tabela com ação por CVE em §1.11.
10. **Adicionar verificação de boot** que loga WARN se `current_prompt_id` do worker não contiver os marcadores KSIR. Transforma a falha silenciosa do bloqueio 1 em falha visível. §4, mitigação 4.
11. **Adicionar teste de integração de evidência** após o #8: tarefa com captura ligada → tool via proxy rotativo → afirmar que `traffic_search` retorna o registro. Única forma de pegar a regressão do item 7. §4, mitigação 6.
12. **`RankArtifacts(taskID)`** no módulo #1 — classificar por valor (densidade de segredo, reutilizabilidade imediata). O v1 lista coleta e empacotamento mas não triagem; dump de 40 GB vale menos que 200 linhas de `web.config`. §2.6 item 51.
13. **Nó de `identity` no asset graph.** Nenhum módulo do v1 cria tipo de nó para identidade, e §2.2 inteira depende disso (e-mail → credencial vazada → tenant → sessão). Decidir schema.
14. **Repositório de conhecimento vetorial (RAG)** entre workers e entre tarefas. §2.8 item 61 aponta como a diferença estrutural entre o ARTEX e as plataformas de ponta. Entrar no v2 como módulo ou como item de roadmap futuro — mas precisa estar **decidido**, não omitido.

---

## 6. Estrutura exigida do v2

O executor não pode ter que interpretar. Cada módulo do v2 carrega:

1. **Objetivo** — uma frase.
2. **Dependências** — módulos que precisam estar prontos, com a aresta explicada.
3. **Arquivos** — tabela `AÇÃO | caminho exato | o que muda`. Todo caminho **verificado como existente** (se for criação, marcado CRIAR). Zero fantasma.
4. **Símbolos** — função/const/linha real, não aproximada. Se a linha puder deslocar, citar o símbolo.
5. **Contratos** — assinatura Go completa para função nova, DDL completo para tabela nova, schema JSON para tool nova.
6. **Testes** — quais arquivos de teste existentes quebram, qual o novo marcador que substitui o antigo, e qual teste novo é adicionado.
7. **Critério de aceite mensurável** — comando executável + saída esperada. Não "deve funcionar".
8. **Rollback** — como desfazer. Migração de prompt tem histórico de versão por design; os outros módulos precisam de plano equivalente.

**Formato:** um módulo por seção, numeração estável (manter #0, #0b, #1–#10, #3b). Tabela-índice no topo com módulo → fase → dependências → complexidade. Ordem de execução explícita com as arestas.

---

## 7. Verificação

Comandos reais do repositório. Nenhum é suposição — estão no v1 e no Dockerfile/build.sh.

```
CGO_ENABLED=0 go build -tags embedui -o artex ./cmd/artex
go test ./...
cd web && npm run build:static
```

Mais o teste funcional de ponta: stack Docker sobe, tarefa de exploração é criada, comportamento novo é observado.

**Cada fase do v2 precisa fechar com os três comandos verdes** antes da próxima começar. `go test ./...` vai falhar nos seed tests até a reescrita de marcadores estar feita — isso é esperado e está no inventário (§1.12 do documento de auditoria).

O v2 deve listar, por módulo, **qual subconjunto de testes** é afetado, para o executor não rodar a suíte inteira a cada passo.

---

## 8. Guardrails

- **Nunca escrever em `main`/`master`.** Worktree própria por ticket.
- Sem push, merge, deploy ou mudança de segredo sem autorização explícita.
- **AGPL-3.0.** O ARTEX é fork BaaRai do Autumn-27/ARTEX. Modificações permanecem sob a mesma licença.
- `SKIP` e `PASS_WITH_WARNINGS` não contam como evidência de conclusão.
- Há um `AGENTS.md` em `C:\Users\error\AGENTS.md` (fora do repo) que referencia `.axi/agent-scripts.sh`. **Não verificado se esse script existe neste repositório** — checar antes de assumir; se não existir, usar os comandos da §7.
- **Escopo autorizado é inegociável.** Ver bloqueio B5. A recomendação da auditoria é manter o trilho de escopo mesmo desarmando o resto do intercept.
- Stack confirmada: Go 1.26.3 monolítico + Next.js embutido via `go:embed` + PostgreSQL 16 + norma SDK v0.4.4. Build de produção em `python:3.12-slim-bookworm`. Host do operador: Windows 11 x64, Parrot OS em WSL disponível.

---

## 9. Suggested skills

Sequência recomendada para esta tarefa:

1. **`17b-spec-tecnica`** — o trabalho central. Congela contratos, tipos, máquinas de estado, taxonomia de erro e critérios de aceite mensuráveis antes do build. É exatamente o que a §6 pede. Rodar primeiro.
2. **`22-validador`** — Paranoid SRE. Stress-testa o blueprint contra o código real e gera checklist atômico. Rodar **depois** do v2 escrito, para pegar qualquer fantasma novo que tenha escapado. É a defesa contra o erro que originou esta consolidação.
3. **`33-auditor`** — QA gate + verificador anti-alucinação, com detecção de phantom. Terceira passada, focada em afirmar que todo caminho e símbolo do v2 existe.
4. **`32-db-architect`** — para as reconciliações de schema: `exfil_artifacts` vs. `task_credentials` vs. tabela do #9, mais as novas `c2_sessions`/`c2_implants`, mais o nó de `identity`. `db/schema.sql` tem 1485 linhas; não mexer às cegas.
5. **`09-research`** — para fechar o bloqueio B6 (CVE-2026-109618/109619) contra fonte primária, e para validar as CVEs novas que entrarem no #10.
6. **`29-tdd`** / **`28-tester`** — para a reescrita dos 6 seed tests de prompt. Não é teste novo, é trocar marcador mantendo a garantia.

**Depois desta consolidação**, o v2 vai para:
- **`26-executor`** — Surgical Code Surgeon, read-before-write, verify-after-write, zero placeholder. É o agente que implementa.
- **`01-map`** — se o executor precisar de mapa fresco da estrutura antes de começar.

Não usar `20-design-interface` nem `21-prototype` aqui: as decisões de interface já foram tomadas na auditoria (`C2Backend`, matriz de perfil, níveis de cleanup). Rediscutir é retrabalho.

---

## 10. O que NÃO fazer

- **Não re-auditar.** A §1 do documento de auditoria já verificou arquivo por arquivo. Repetir desperdiça o orçamento.
- **Não reimplementar evasão dentro do ARTEX.** Loader, cripter, injetor, syscalls, hollowing — tudo isso vive no implant do painel C2 (Rust). O ARTEX **seleciona perfil** e **faz handoff**. Ver §1.7 e §2.9. A distinção é executar vs. conhecer: o catálogo de evasão (§2.9) existe para virar parâmetro de contrato, não código Go.
- **Não pesquisar técnica nova** sem checar primeiro se já está nos 88 complementos da §2. A cobertura é OSINT de infraestrutura, OSINT de identidade, nuvem/CI-CD, contorno de WAF, exfiltração, persistência, arquitetura agêntica, evasão de EDR, anti-forense e matriz de frameworks C2.
- **Não editar `planos/KSIR-WEAPONIZATION.md`.** O v1 fica como registro. Escrever `planos/KSIR-WEAPONIZATION-v2.md`.
- **Não começar implementação.** Esta tarefa entrega **plano**, não código. O executor vem depois.
- **Não inventar caminho de arquivo.** Se não foi verificado, não entra. Foi assim que o v1 quebrou.

---

## 11. Definição de pronto

O v2 está pronto para o executor quando:

1. Todo caminho de arquivo citado existe no repositório **ou** está marcado CRIAR.
2. Todo símbolo citado (função, const, tabela, endpoint) foi confirmado por leitura do código.
3. Nenhum intervalo de linha é usado como identificador primário — símbolo primeiro, linha como apoio.
4. Os 7 bloqueios da §4 têm resposta registrada, ou o módulo afetado está explicitamente marcado **BLOQUEADO — aguardando operador** com o impacto declarado.
5. As 14 pontas soltas da §5 estão resolvidas no texto do plano.
6. Os 11 módulos (#0, #0b, #1, #2, #3, #3b, #4, #5, #6, #7, #8, #9, #10) têm as 8 seções da §6 preenchidas.
7. A ordem de execução tem as arestas declaradas e justificadas.
8. `22-validador` e `33-auditor` rodaram sobre o v2 e não retornaram phantom nem contrato subespecificado.
