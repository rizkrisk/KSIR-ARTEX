package agent

import (
	"context"
	"encoding/json"

	actool "github.com/Autumn-27/norma/tool"
)

// DeployPayloadArgs are the parameters for the deploy_payload tool.
type DeployPayloadArgs struct {
	TargetURL      string `json:"target_url"`
	PayloadType    string `json:"payload_type"` // webshell/reverse_shell/bind_shell
	PayloadBody    string `json:"payload_body"`
	DeliveryMethod string `json:"delivery_method"` // upload/sqli/lfi/rce
}

// EstablishPersistenceArgs parameters for establish_persistence.
type EstablishPersistenceArgs struct {
	Method  string `json:"method"` // cron/registry/service/startup
	Command string `json:"command"`
	Target  string `json:"target"`
}

// ExfiltrateArgs parameters for exfiltrate.
type ExfiltrateArgs struct {
	TargetPath string `json:"target_path"`
	Pattern    string `json:"pattern"`
	MaxSizeMB  int    `json:"max_size_mb"`
}

// LateralMoveArgs parameters for lateral_move.
type LateralMoveArgs struct {
	Target     string `json:"target"`
	Method     string `json:"method"` // wmi/winrm/smb/ssh/psexec/dcom
	Credential string `json:"credential_id"`
}

// toolDeployPayload implements the deploy_payload agent tool.
func toolDeployPayload() actool.CoreTool {
	return writeTool(
		"deploy_payload",
		"Deploy an offensive payload to a target URL via the specified delivery method.",
		obj(map[string]any{
			"target_url":      str("Target URL to deliver the payload to"),
			"payload_type":    str("Type: webshell, reverse_shell, bind_shell"),
			"payload_body":    str("The payload source code or content"),
			"delivery_method": str("Delivery: upload, sqli, lfi, rce"),
		}, "target_url", "payload_type", "payload_body", "delivery_method"),
		func(_ context.Context, in json.RawMessage) (actool.Result, error) {
			var args DeployPayloadArgs
			if err := json.Unmarshal(in, &args); err != nil {
				return actool.Errorf("invalid params: " + err.Error()), nil
			}
			return actool.Text("payload deployment initiated: type=" + args.PayloadType + " method=" + args.DeliveryMethod + " target=" + args.TargetURL), nil
		},
	)
}

// toolEstablishPersistence implements the establish_persistence agent tool.
func toolEstablishPersistence() actool.CoreTool {
	return writeTool(
		"establish_persistence",
		"Establish persistence on a target host via the specified method.",
		obj(map[string]any{
			"method":  str("Persistence method: cron, registry, service, startup"),
			"command": str("Command or payload to persist"),
			"target":  str("Target host or process"),
		}, "method", "command", "target"),
		func(_ context.Context, in json.RawMessage) (actool.Result, error) {
			var args EstablishPersistenceArgs
			if err := json.Unmarshal(in, &args); err != nil {
				return actool.Errorf("invalid params: " + err.Error()), nil
			}
			return actool.Text("persistence established: method=" + args.Method + " target=" + args.Target), nil
		},
	)
}

// toolExfiltrate implements the exfiltrate agent tool.
func toolExfiltrate() actool.CoreTool {
	return writeTool(
		"exfiltrate",
		"Collect and stage data for exfiltration from a target path.",
		obj(map[string]any{
			"target_path": str("Path to collect from"),
			"pattern":     str("File pattern to match (e.g. *.config, *.env)"),
			"max_size_mb": intp("Maximum total size in MB to collect"),
		}, "target_path"),
		func(_ context.Context, in json.RawMessage) (actool.Result, error) {
			var args ExfiltrateArgs
			if err := json.Unmarshal(in, &args); err != nil {
				return actool.Errorf("invalid params: " + err.Error()), nil
			}
			return actool.Text("exfiltration staged: path=" + args.TargetPath + " pattern=" + args.Pattern), nil
		},
	)
}

// toolLateralMove implements the lateral_move agent tool.
func toolLateralMove() actool.CoreTool {
	return writeTool(
		"lateral_move",
		"Attempt lateral movement to a target using the specified method and credential.",
		obj(map[string]any{
			"target":        str("Target host"),
			"method":        str("Movement method: wmi, winrm, smb, ssh, psexec, dcom"),
			"credential_id": str("Reference to task_credentials ID"),
		}, "target", "method"),
		func(_ context.Context, in json.RawMessage) (actool.Result, error) {
			var args LateralMoveArgs
			if err := json.Unmarshal(in, &args); err != nil {
				return actool.Errorf("invalid params: " + err.Error()), nil
			}
			return actool.Text("lateral move attempted: " + args.Method + " → " + args.Target), nil
		},
	)
}
