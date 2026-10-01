//go:build windows

package builtin

// diskUsage is Linux-only for now; Windows reports a dash.
func diskUsage() string { return "—" }
