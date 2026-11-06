//go:build windows

package daemon

func processAlive(pid int) bool {
	return pid > 0
}
