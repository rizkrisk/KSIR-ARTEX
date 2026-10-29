=== CONTEXTO DO PROJETO ===

Projeto: ARTEX (fork BaaRai do Autumn-27/ARTEX, licenca AGPL-3.0)
Local: C:\Users\error\Desktop\ARTEX-main\ARTEX-main\
Stack: Go 1.26.3 monolitico + Next.js embutido (go:embed) + PostgreSQL 16
SDK: norma v0.4.4 (agentes LLM multi-role)
Arquitetura: grafos duplos — asset graph (global) + exploration graph (por tarefa)
Agentes: Planner (gera intencoes), Workers (executam, N em paralelo), MainAgent (chat humano)
362 arquivos .go, 186 .tsx/.ts, 20 skills em skills/
Build: CGO_ENABLED=0 go build -tags embedui -o artex ./cmd/artex
Frontend: cd web && npm run build:static
Prod: Docker (Dockerfile python:3.12-slim-bookworm + binario Go)
Sistema: Windows 11 x64 host, Parrot OS WSL disponivel

=== PLANO ATUAL (planos/KSIR-WEAPONIZATION.md) ===

9 modulos de weaponizacao planejados, organizados em 3 fases:

FASE 1 — Fundacao:
  #2 REMOVER RATE LIMITERS — zerar throttles em server/notifier.go,
     agent/provider.go:376, notify/*.go, llmpool/health.go, frontend de LLM/notify
  #3 WEAPONIZED EXPLOITATION — reescrever workerDefaultTmpl (agent/worker.go:226-346)
     com doutrina ofensiva, remover adversarial gate/finding quality gates do
     report_finding (agent/tools.go:1346), adicionar tools: deploy_payload,
     establish_persistence, exfiltrate, lateral_move, dump_credentials
  #5 PHP2SHELL CLUSTER — remover gates do skills/php2shell/SKILL.md, liberar chains
     2+3 para remoto, criar scanner paralelo + payload polimorfico, pipeline de cluster

FASE 2 — Exploracao:
  #1 RELATORIOS→EXFILTRACAO — substituir report/report.go por exfil/exfil.go,
     endpoint /api/report→/api/exfil, aba report-tab.tsx→exfil-tab.tsx, tool exfil_data
  #4 DB + EXPLOITS RECENTES — novas tabelas exploit_payloads, exploit_chains,
     exfil_artifacts, seed com CVEs 2024-2026 (PHP CGI, Tomcat, LDAP, netfilter UAF,
     Log4Shell), 5 skills novas: rce-direct, sqli-to-rce, file-upload-weaponize,
     deserialize-chain, ssrf-to-rce
  #6 TOKENIZADOR DE ERROS + PAYLOAD POLIMORFICO — agent/polyengine.go + errtoken.go,
     sub-agente que analisa erros do alvo e reescreve payload em tempo real,
     templates por linguagem (PHP/Python/Go/Java/C#/PowerShell)

FASE 3 — Evasao:
  #7 OFUSCACAO DINAMICA + INJECAO EM MEMORIA — evasion/ completo: Garble (Go),
     PyArmor (Python), Reflective DLL Injection, Process Hollowing, direct syscalls
     com hashed imports, beacon HTTP/HTTPS/DNS, shellcode via donut/sRDI
  #8 GRAFO DE PROXY DINAMICO — proxygraph/: AWS EC2 spot + Azure ACI efemeras,
     proxies residenciais (BrightData), TOR circuits, rotacao de IP/UA/headers/API key
     por requisicao, exfiltracao embutida em trafego HTTPS mimetizando MS365/Google
  #9 MOVIMENTACAO LATERAL + PRIV-ESC — skills/lateral-movement/ + tools_lateral.go,
     LSASS dump→Mimikatz→DCSync→Golden Ticket, arvore de decisao autonoma ate DC

=== TUA TAREFA ===

1. AUDITAR O PLANO
   Le o arquivo planos/KSIR-WEAPONIZATION.md completo (use read_file).
   Para cada um dos 9 modulos, verifique:
   - Os arquivos listados para modificar/criar realmente existem onde o plano diz?
   - As funcoes e linhas citadas (ex: worker.go:226-346, provider.go:376) batem
     com o codigo real?
   - Ha alguma dependencia entre modulos que nao foi considerada?
   - Algum modulo esta sub-especificado ou vago demais?
   - Ha contradicoes entre modulos?
   - O plano cobre remocao completa dos intercept gates (intercept.go,
     constraints.go, asset_intercept.go)?
   - Falta tratar o agente "reporter" existente que e triggerado apos
     report_finding? Ele quebra com as mudancas do modulo #1 e #3?

2. PESQUISAR COMPLEMENTOS TATICOS AVANCADOS (use web_search)
   Busque tecnicas ofensivas e evasivas MODERNAS (2025-2026) que faltam no plano:
   - Novas tecnicas de evasao de EDR/XDR alem das listadas (call stack spoofing,
     hardware breakpoints, Vectored Exception Handling, ETW bypass via
     EtwEventWrite, kernel callbacks)
   - Tecnicas de persistencia alem das mencionadas (WMI event subscriptions,
     BITS jobs, LSA authentication packages, time providers, Print Monitors)
   - Tecnicas de C2 moderno (DNS-over-HTTPS, WebSocket tunneling, MQTT,
     QUIC/HTTP3, dead drop resolvers, domain fronting atualizado)
   - Tecnicas de exfiltracao stealth (DNS tunneling, ICMP tunneling,
     steganografia em imagens, chunked HTTP responses, WebRTC data channels)
   - Tecnicas de bypass de WAF 2026 (HTTP request smuggling v2, charset
     confusion, JSON/XML parser differentials, hop-by-hop header abuse)
   - Tecnicas de abuso de cloud (IMDS v2 bypass, AWS Lambda layers, Azure
     managed identities, GCP service account chaining, CI/CD pipeline injection,
     Terraform state exfiltration)
   - Frameworks/tools recentes que deveriam ser integrados (Havoc C2, Mythic,
     Sliver, BRC4, Nighthawk, Cobalt Strike 4.10+ artifacts)
   - Tecnicas de anti-forense (timestomp, USN journal manipulation, event
     log wiping seletivo, prefetch suppression, MFT hole)
   - Tecnicas de phishing moderno (evilginx 4.x, adversary-in-the-middle com
     WebAuthn bypass, OAuth device code phishing, MFA fatigue automation)

3. REPORTAR
   Gera um arquivo planos/KSIR-AUDIT-COMPLEMENTO.md contendo:
   - Secao AUDITORIA: tabela de achados por modulo (status: OK/FALHA/INCOMPLETO,
     detalhe do problema, sugestao de correcao)
   - Secao COMPLEMENTOS: lista de tecnologias/tecnicas avancadas que deveriam ser
     adicionadas ao plano, com explicacao de 1 paragrafo por item, e qual modulo
     existente ele complementa
   - Secao PRIORIDADES: top 10 itens que deveriam entrar no roadmap (entre
     correcoes e complementos), ordenados por impacto tatico
   - Secao RISCO: 3 maiores riscos tecnicos do plano atual e como mitigar

=== REGRAS ===
- Toda afirmacao sobre arquivos ou funcoes especificas deve ser verificada lendo
  o codigo real primeiro
- Pesquise antes de recomendar — nao invente tecnicas
- Seja cirurgico: aponte exatamente o que esta errado e como corrigir
- Nao reescreva o plano inteiro; apenas aponte lacunas e correcoes
- O arquivo de saida e planos/KSIR-AUDIT-COMPLEMENTO.md
- Responda em portugues BR direto, sem enrolacao