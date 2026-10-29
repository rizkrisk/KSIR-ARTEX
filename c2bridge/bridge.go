package c2bridge

import "context"

// C2Backend — RAT-without-API version.
// No RequestImplant, ListListeners, Exec, Cleanup (RAT doesn't expose those).
type C2Backend interface {
	Stage(ctx context.Context, arch string) (localPath string, err error)
	Deliver(ctx context.Context, vector DeliveryVector, binaryPath string) error
}

// DeliveryVector describes where and how to stage the implant on the target.
type DeliveryVector struct {
	Type       string // "webshell_upload"|"echo_b64"|"certutil"|"bitsadmin"|"wget"|"curl"|"smb"
	ShellURL   string // URL of existing webshell if webshell_upload
	TargetPath string // destination path on target
}
