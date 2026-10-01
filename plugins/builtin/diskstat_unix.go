//go:build !windows

package builtin

import (
	"os"
	"syscall"
)

// diskUsage reports "used / total (n%)" for the working directory.
func diskUsage() string {
	var fs syscall.Statfs_t
	wd, err := os.Getwd()
	if err != nil || syscall.Statfs(wd, &fs) != nil {
		return "—"
	}
	total := int64(fs.Blocks) * int64(fs.Bsize)
	free := int64(fs.Bavail) * int64(fs.Bsize)
	return pct(total-free, total)
}
