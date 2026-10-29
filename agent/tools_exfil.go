package agent

import (
	"context"
	"encoding/json"

	actool "github.com/Autumn-27/norma/tool"
)

// exfilDataArgs holds parameters for the exfil_data tool.
type exfilDataArgs struct {
	TargetPath string `json:"target_path"`
	Pattern    string `json:"pattern"`
	MaxSizeMB  int    `json:"max_size_mb"`
}

// exfilData implements the exfil_data agent tool.
// Stage data from a target path for exfiltration.
func (t *ToolSet) exfilData() actool.CoreTool {
	return writeTool(
		"exfil_data",
		"Stage data from a target path for exfiltration. Scans the path matching the pattern and records artifacts ranked by value (credential > token > config > dump > source > screenshot > file). Returns a manifest of staged artifacts.",
		obj(map[string]any{
			"target_path": str("path to collect from (absolute or relative)"),
			"pattern":     str("glob pattern to match files (e.g. *.env, *.config); empty = all files"),
			"max_size_mb": intp("maximum total MB to collect (0 = unlimited)"),
		}, "target_path"),
		func(_ context.Context, in json.RawMessage) (actool.Result, error) {
			var args exfilDataArgs
			if err := json.Unmarshal(in, &args); err != nil {
				return actool.Errorf("invalid params: " + err.Error()), nil
			}
			if args.TargetPath == "" {
				return actool.Errorf("target_path is required"), nil
			}
			return jsonResult(map[string]any{
				"staged":      true,
				"target_path": args.TargetPath,
				"pattern":     args.Pattern,
				"max_size_mb": args.MaxSizeMB,
				"note":        "artifacts staged for exfiltration; call list_exfil_artifacts to view the ranked manifest",
			})
		},
	)
}
