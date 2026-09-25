package main

import (
	"os"
	"os/exec"
)

type Dbus struct {
	Cmd        *exec.Cmd
	SocketPath string
}

func StartDbusproxy(proxyExe string, proxyArgs []string) (dbus Dbus) {
	cmd := exec.Command(proxyExe, proxyArgs...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	dbus.Cmd = cmd
	if len(proxyArgs) >= 2 {
		dbus.SocketPath = proxyArgs[1]
		os.Remove(dbus.SocketPath)
	}

	if err := cmd.Start(); err != nil {
		panic(err)
	}

	return
}

func (dbus *Dbus) WaitUntilStartup() {
	if dbus.SocketPath != "" {
		waitUntilFileAppears(dbus.SocketPath)
	}
}

func (dbus *Dbus) Close() {
	// In detached mode, dbus-proxy lifecycle is managed by the systemd cgroup
}

