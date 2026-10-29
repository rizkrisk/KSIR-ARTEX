#!/usr/bin/env python3
"""
ARTEX CVE Auto-Updater
Busca CVEs críticos/altos modernos (NVD + ExploitDB + GitHub Advisories)
e gera/atualiza skills no diretório skills/cve-live/ do ARTEX.

Fontes:
  - NVD API v2 (nvd.nist.gov) — sem key, 5 req/30s
  - ExploitDB CSV (gitlab.com/exploit-database) — via raw download
  - GitHub Advisory Database (api.github.com) — público sem key
  - CISA KEV (known exploited vulns) — JSON público

Uso: python3 scripts/update-cves.py
Cron: 0 6 * * * cd /path/to/artex && python3 scripts/update-cves.py >> /tmp/artex-cve-update.log 2>&1
"""

import json
import os
import sys
import time
import textwrap
from datetime import datetime, timedelta, timezone
from pathlib import Path

import requests

# ── config ────────────────────────────────────────────────────────────────────
ARTEX_ROOT   = Path(__file__).parent.parent
SKILLS_DIR   = ARTEX_ROOT / "skills"
CVE_SKILL    = SKILLS_DIR / "cve-live"
CVE_SKILL.mkdir(parents=True, exist_ok=True)

HEADERS = {"User-Agent": "ARTEX-CVE-Updater/1.0"}
CVSS_MIN = 7.0          # pega High + Critical
DAYS_BACK = 730         # 2 anos de NVD
MAX_PER_SOURCE = 200    # limite por fonte
FULL_KEV = True         # pega TODO o histórico do CISA KEV (~1200 entradas)

session = requests.Session()
session.headers.update(HEADERS)

def log(msg): print(f"[{datetime.now().strftime('%H:%M:%S')}] {msg}", flush=True)

# ── fontes ─────────────────────────────────────────────────────────────────────

def fetch_nvd() -> list[dict]:
    """NVD API v2 — CRITICAL + HIGH dos últimos DAYS_BACK dias, paginado."""
    log(f"NVD: buscando CVEs (últimos {DAYS_BACK} dias, CRITICAL + HIGH)...")
    now  = datetime.now(timezone.utc)
    past = now - timedelta(days=DAYS_BACK)
    results = []

    # NVD limita janelas a 120 dias — divide em chunks
    windows = []
    cursor = past
    while cursor < now:
        end = min(cursor + timedelta(days=119), now)
        windows.append((cursor, end))
        cursor = end + timedelta(seconds=1)
    log(f"NVD: {len(windows)} janelas de 120 dias para cobrir {DAYS_BACK} dias")

    for severity in ["CRITICAL", "HIGH"]:
        for win_start, win_end in windows:
          start_idx = 0
          while True:
            params = {
                "pubStartDate":   win_start.strftime("%Y-%m-%dT00:00:00.000"),
                "pubEndDate":     win_end.strftime("%Y-%m-%dT23:59:59.999"),
                "cvssV3Severity": severity,
                "resultsPerPage": 100,
                "startIndex":     start_idx,
            }
            try:
                r = session.get("https://services.nvd.nist.gov/rest/json/cves/2.0",
                                params=params, timeout=25)
                r.raise_for_status()
                data = r.json()
                batch = data.get("vulnerabilities", [])
                total_results = data.get("totalResults", 0)
                log(f"  NVD {severity} offset={start_idx}/{total_results}: {len(batch)} itens")

                for item in batch:
                    cve    = item.get("cve", {})
                    cve_id = cve.get("id", "")
                    desc   = next(
                        (d["value"] for d in cve.get("descriptions", []) if d["lang"] == "en"),
                        "No description"
                    )
                    metrics = cve.get("metrics", {})
                    score, vector = 0.0, ""
                    for key in ["cvssMetricV31", "cvssMetricV30", "cvssMetricV2"]:
                        if key in metrics and metrics[key]:
                            m      = metrics[key][0].get("cvssData", {})
                            score  = m.get("baseScore", 0.0)
                            vector = m.get("vectorString", "")
                            break
                    if score < CVSS_MIN:
                        continue
                    refs = [r2["url"] for r2 in cve.get("references", [])[:3]]
                    weaknesses = []
                    for w in cve.get("weaknesses", []):
                        for d in w.get("description", []):
                            if d["lang"] == "en":
                                weaknesses.append(d["value"])
                    results.append({
                        "id": cve_id, "score": score, "vector": vector,
                        "desc": desc[:300], "refs": refs,
                        "weaknesses": weaknesses,
                        "source": "NVD",
                        "published": cve.get("published", "")[:10],
                    })

                start_idx += len(batch)
                if not batch or start_idx >= min(total_results, MAX_PER_SOURCE):
                    break
                time.sleep(7)

            except Exception as e:
                log(f"NVD {severity} {win_start.date()} ERROR: {e}")
                break

          time.sleep(7)  # entre janelas

    log(f"NVD: {len(results)} CVEs High+Critical encontrados")
    return results


def fetch_cisa_kev() -> list[dict]:
    """CISA KEV — Known Exploited Vulnerabilities — HISTÓRICO COMPLETO."""
    log("CISA KEV: buscando catálogo COMPLETO de vulnerabilidades exploradas...")
    results = []
    try:
        r = session.get(
            "https://www.cisa.gov/sites/default/files/feeds/known_exploited_vulnerabilities.json",
            timeout=20
        )
        r.raise_for_status()
        vulns = r.json().get("vulnerabilities", [])
        # sem filtro de data quando FULL_KEV=True
        cutoff = "2000-01-01" if FULL_KEV else (datetime.now(timezone.utc) - timedelta(days=DAYS_BACK)).strftime("%Y-%m-%d")
        for v in vulns:
            added = v.get("dateAdded", "")
            if added < cutoff:
                continue
            results.append({
                "id": v.get("cveID", ""),
                "score": 9.0,   # KEV = explorado = assume crítico
                "vector": "",
                "desc": f"[ACTIVELY EXPLOITED] {v.get('shortDescription', '')}",
                "refs": [f"https://nvd.nist.gov/vuln/detail/{v.get('cveID','')}"],
                "weaknesses": [v.get("vulnerabilityName", "")],
                "source": "CISA-KEV",
                "published": added,
                "vendor": v.get("vendorProject", ""),
                "product": v.get("product", ""),
                "action": v.get("requiredAction", ""),
            })
        log(f"CISA KEV: {len(results)} vulns exploradas nos últimos {DAYS_BACK} dias")
    except Exception as e:
        log(f"CISA KEV ERROR: {e}")
    return results[:MAX_PER_SOURCE]


def fetch_github_advisories() -> list[dict]:
    """GitHub Advisory DB — foco em web/backend: npm, pip, go, rubygems."""
    log("GitHub Advisories: buscando...")
    results = []
    ecosystems = ["NPM", "PIP", "GO", "RUBYGEMS", "COMPOSER"]
    for eco in ecosystems:
        try:
            url = f"https://api.github.com/advisories?ecosystem={eco}&severity=critical&per_page=10"
            r = session.get(url, timeout=15)
            if r.status_code == 200:
                for adv in r.json():
                    cve_ids = adv.get("cve_id") or ""
                    results.append({
                        "id": cve_ids or adv.get("ghsa_id", ""),
                        "score": 9.5 if adv.get("severity") == "critical" else 7.5,
                        "vector": "",
                        "desc": adv.get("summary", "")[:300],
                        "refs": [adv.get("html_url", "")],
                        "weaknesses": [adv.get("cwe_ids", [""])[0]] if adv.get("cwe_ids") else [],
                        "source": f"GitHub/{eco}",
                        "published": (adv.get("published_at") or "")[:10],
                        "package": adv.get("vulnerabilities", [{}])[0].get("package", {}).get("name", ""),
                        "patched": adv.get("vulnerabilities", [{}])[0].get("first_patched_version", ""),
                    })
            time.sleep(1)
        except Exception as e:
            log(f"GitHub/{eco} ERROR: {e}")
    log(f"GitHub Advisories: {len(results)} encontrados")
    return results[:MAX_PER_SOURCE]


def fetch_exploitdb_recent() -> list[dict]:
    """ExploitDB CSV — exploits publicados recentemente (web focus)."""
    log("ExploitDB: buscando exploits recentes...")
    results = []
    try:
        # ExploitDB publica CSV atualizado diariamente
        url = "https://gitlab.com/exploit-database/exploitdb/-/raw/main/files_exploits.csv"
        r = session.get(url, timeout=30)
        r.raise_for_status()
        lines = r.text.splitlines()
        cutoff = (datetime.now(timezone.utc) - timedelta(days=DAYS_BACK)).strftime("%Y-%m-%d")
        web_types = {"webapps", "web"}
        count = 0
        for line in reversed(lines[1:]):  # mais recentes no final
            parts = line.split(",")
            if len(parts) < 6:
                continue
            edb_id, desc, date, author, platform, type_ = parts[0], parts[1], parts[2], parts[3], parts[4], parts[5]
            if date < cutoff:
                break
            if type_.strip().lower() not in web_types:
                continue
            results.append({
                "id": f"EDB-{edb_id.strip()}",
                "score": 8.0,
                "vector": "",
                "desc": desc.strip().strip('"')[:300],
                "refs": [f"https://www.exploit-db.com/exploits/{edb_id.strip()}"],
                "weaknesses": [],
                "source": "ExploitDB",
                "published": date.strip(),
                "platform": platform.strip(),
            })
            count += 1
            if count >= MAX_PER_SOURCE:
                break
        log(f"ExploitDB: {len(results)} web exploits recentes")
    except Exception as e:
        log(f"ExploitDB ERROR: {e}")
    return results


# ── gerador de skill ───────────────────────────────────────────────────────────

def severity_label(score: float) -> str:
    if score >= 9.0: return "CRÍTICO"
    if score >= 7.0: return "ALTO"
    if score >= 4.0: return "MÉDIO"
    return "BAIXO"


def build_skill_md(all_cves: list[dict]) -> str:
    """Gera o SKILL.md consolidado para o agente ARTEX."""
    now = datetime.now(timezone.utc).strftime("%Y-%m-%d %H:%M UTC")
    total = len(all_cves)

    # agrupa por fonte
    by_source: dict[str, list] = {}
    for c in all_cves:
        src = c["source"]
        by_source.setdefault(src, []).append(c)

    lines = [
        f"---",
        f"name: cve-live",
        f'description: "Base de CVEs e exploits ativamente explorados. Atualizada em {now}. {total} entradas. Consulte esta skill em todo engagement para identificar vetores modernos aplicáveis ao alvo."',
        f"---",
        f"",
        f"# CVE Live — Base de Vulnerabilidades Modernas",
        f"",
        f"**Atualizado:** {now}  ",
        f"**Total:** {total} entradas  ",
        f"**Fontes:** {', '.join(by_source.keys())}",
        f"",
        f"---",
        f"",
        f"## Instruções de uso para o agente",
        f"",
        f"1. Ao receber um alvo, identifique: stack (linguagem, framework, versão, OS, cloud)",
        f"2. Filtre esta lista pelos CVEs que se aplicam à stack identificada",
        f"3. Priorize entradas marcadas `[KEV]` — estão sendo exploradas ativamente in-the-wild",
        f"4. Para cada CVE aplicável, verifique a referência e adapte o payload ao alvo",
        f"5. Encadeie vulnerabilidades quando possível (ex: LFI → log poisoning → RCE)",
        f"",
        f"---",
        f"",
    ]

    # CISA KEV primeiro (mais urgente)
    kev = by_source.get("CISA-KEV", [])
    if kev:
        lines += [
            f"## 🔴 CISA KEV — Exploração Ativa Confirmada ({len(kev)})",
            f"",
            f"Estas vulnerabilidades têm exploração in-the-wild **confirmada pelo CISA**.",
            f"São as mais prioritárias em qualquer engagement.",
            f"",
        ]
        for c in sorted(kev, key=lambda x: x["published"], reverse=True):
            vendor  = c.get("vendor", "")
            product = c.get("product", "")
            target  = f"{vendor} / {product}" if vendor else ""
            lines += [
                f"### `{c['id']}` [KEV] {f'— {target}' if target else ''}",
                f"**Data:** {c['published']}  ",
                f"**Descrição:** {c['desc']}",
            ]
            if c.get("action"):
                lines.append(f"**Ação requerida:** {c['action']}")
            if c["refs"]:
                lines.append(f"**Ref:** {c['refs'][0]}")
            lines.append("")

    # NVD críticos
    nvd = by_source.get("NVD", [])
    if nvd:
        lines += [
            f"## 🟠 NVD — CVEs Críticos Recentes ({len(nvd)})",
            f"",
        ]
        for c in sorted(nvd, key=lambda x: x["score"], reverse=True):
            sev = severity_label(c["score"])
            lines += [
                f"### `{c['id']}` — CVSS {c['score']} [{sev}]",
                f"**Data:** {c['published']}  ",
            ]
            if c["weaknesses"]:
                lines.append(f"**CWE:** {', '.join(c['weaknesses'][:2])}")
            lines += [
                f"**Descrição:** {c['desc']}",
            ]
            if c["refs"]:
                lines.append(f"**Refs:** " + " · ".join(c["refs"][:2]))
            lines.append("")

    # GitHub Advisories (pacotes web)
    for eco_key in [k for k in by_source if k.startswith("GitHub/")]:
        eco_list = by_source[eco_key]
        eco_name = eco_key.replace("GitHub/", "")
        lines += [f"## 📦 GitHub Advisories — {eco_name} ({len(eco_list)})", ""]
        for c in eco_list:
            pkg     = c.get("package", "")
            patched = c.get("patched", "")
            lines += [
                f"### `{c['id']}` — {pkg or c['desc'][:60]}",
                f"**CVSS:** {c['score']}  **Data:** {c['published']}",
            ]
            if patched:
                lines.append(f"**Fix:** >= {patched}")
            lines += [f"**Descrição:** {c['desc']}", f"**Ref:** {c['refs'][0] if c['refs'] else ''}", ""]

    # ExploitDB web
    edb = by_source.get("ExploitDB", [])
    if edb:
        lines += [f"## 💥 ExploitDB — Web Exploits Recentes ({len(edb)})", ""]
        for c in edb:
            lines += [
                f"### `{c['id']}` — {c['desc'][:80]}",
                f"**Data:** {c['published']}  **Plataforma:** {c.get('platform','')}",
                f"**POC:** {c['refs'][0] if c['refs'] else ''}",
                "",
            ]

    # rodapé
    lines += [
        "---",
        "",
        "## Técnicas Complementares (sempre válidas independente de CVE)",
        "",
        "### Authentication",
        "- JWT `alg:none` / RS256→HS256 confusion / weak secret brute-force (rockyou)",
        "- OAuth: device_code phishing, implicit flow token theft, state CSRF",
        "- SAML: XML signature wrapping, XXE via SAML, comment injection",
        "",
        "### Injection",
        "- SQLi: boolean blind, error-based, time-based (SLEEP / RANDOMBLOB), UNION dump, stacked queries",
        "- SSTI: Jinja2 `{{7*7}}`, Twig `{{7*'7'}}`, FreeMarker `${7*7}`, Pebble, Velocity",
        "- XXE: external entity, blind OOB via DNS, SVG upload, XLSX/DOCX",
        "- SSRF: AWS IMDS v1 `169.254.169.254`, Azure `169.254.169.254/metadata`, GCP `metadata.google.internal`",
        "",
        "### Business Logic",
        "- IDOR: enumerar IDs sequenciais em /api/users/:id, /api/orders/:id",
        "- Price manipulation: negative qty, float overflow, coupon replay",
        "- Race condition: concurrent requests em operações de saldo/estoque",
        "",
        "### Modern Stack",
        "- GraphQL: introspection dump, batching DoS, field suggestion, alias abuse",
        "- gRPC: reflection endpoint dump, proto file extraction",
        "- WebSocket: protocol injection, cross-site WebSocket hijacking (CSWSH)",
        "- HTTP/2: request smuggling H2.CL / H2.TE",
        "- Prototype pollution: `__proto__` via JSON merge, query string, YAML",
        "",
        "### Cloud / CI-CD",
        "- AWS: IMDS v1 → role credentials → S3/ECR/SSM pivot",
        "- GitHub Actions: `pull_request_target` + fork → secret exfil",
        "- Terraform state: S3/GCS backend sem auth → credentials em plaintext",
        "- OIDC trust abuse: sub condition permissive → cross-tenant impersonation",
        "",
    ]

    return "\n".join(lines)


# ── main ───────────────────────────────────────────────────────────────────────

def main():
    log("=== ARTEX CVE Updater iniciando ===")
    log(f"Destino: {CVE_SKILL}")

    all_cves: list[dict] = []

    # busca em paralelo (sequencial por rate limit)
    all_cves += fetch_cisa_kev()
    all_cves += fetch_nvd()
    all_cves += fetch_github_advisories()
    all_cves += fetch_exploitdb_recent()

    # deduplica por ID
    seen: set[str] = set()
    deduped = []
    for c in all_cves:
        if c["id"] and c["id"] not in seen:
            seen.add(c["id"])
            deduped.append(c)
    all_cves = deduped

    log(f"Total após dedup: {len(all_cves)} entradas")

    # gera skill
    skill_md = build_skill_md(all_cves)
    skill_file = CVE_SKILL / "SKILL.md"
    skill_file.write_text(skill_md, encoding="utf-8")
    log(f"SKILL.md escrito: {skill_file} ({len(skill_md)} chars)")

    # salva JSON de backup
    backup = CVE_SKILL / "cves.json"
    backup.write_text(json.dumps(all_cves, indent=2, ensure_ascii=False), encoding="utf-8")
    log(f"Backup JSON: {backup}")

    log("=== Concluído ===")
    return len(all_cves)


if __name__ == "__main__":
    count = main()
    sys.exit(0 if count > 0 else 1)
