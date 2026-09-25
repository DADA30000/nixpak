package main

import (
	"crypto/md5"
	"encoding/base32"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"

	"github.com/fsnotify/fsnotify"
)

func envOr(name string, or string) string {
	val, found := os.LookupEnv(name)
	if found {
		return val
	} else {
		return or
	}
}

func requiredEnv(name string) string {
	val, found := os.LookupEnv(name)
	if !found || val == "" {
		panic(fmt.Sprintf("environment variable '%s' not set", name))
	}
	return val
}

func instanceId() string {
	var sum = md5.Sum([]byte(strconv.Itoa(os.Getpid())))
	var enc = base32.NewEncoding("0123456789abcdfghijklmnpqrsvwxyz").WithPadding(base32.NoPadding)
	return enc.EncodeToString(sum[:])
}

func waitUntilFileAppears(filename string) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		panic(err)
	}
	defer watcher.Close()

	if err := watcher.Add(filepath.Dir(filename)); err != nil {
		panic(err)
	}

	if _, err := os.Stat(filename); err == nil {
		return
	}

	for {
		select {
		case event := <-watcher.Events:
			if event.Name == filename && event.Op == fsnotify.Create {
				return
			}
		case err := <-watcher.Errors:
			panic(err)
		}
	}
}

func run() error {
	var flatpakMetadata FlatpakMetadata

	conf := readConfig()

	if conf.UseFlatpakMetadata {
		flatpakMetadata.InfoFileTemplate = conf.FlatpakMetadataTemplate
		flatpakMetadata.MetadataDirectory = os.Getenv("XDG_RUNTIME_DIR") + "/.flatpak/nixpak-app-" + instanceId()
		flatpakMetadata.Setup()
	}

	var syncFds []*os.File

	if conf.UseDbusProxy {
		dbus := StartDbusproxy(conf.DbusproxyExe, conf.DbusproxyArgs)
		dbus.WaitUntilStartup()
		syncFds = append(syncFds, dbus.SyncRead)
	}

	if conf.UseSystemDbusProxy {
		systemDbus := StartDbusproxy(conf.DbusproxyExe, conf.SystemDbusproxyArgs)
		systemDbus.WaitUntilStartup()
		syncFds = append(syncFds, systemDbus.SyncRead)
	}

	if conf.UseWaylandProxy {
		waylandProxy := StartWaylandProxy(conf)
		waylandProxy.WaitUntilStartup()
	}

	bwrap := StartBwrap(conf, flatpakMetadata, syncFds...)
	bwrapInfo := bwrap.WaitUntilSandboxReady()

	if conf.UseFlatpakMetadata {
		flatpakMetadata.WriteBwrapInfo(bwrapInfo.Raw)
	}

	if conf.UsePasta {
		StartPasta(conf, bwrapInfo.ChildPid)
	}

	bwrap.NotifySandboxFinished()
	os.Exit(0)

	return nil
}

func main() {
	if err := run(); err != nil {
		if exiterr, ok := err.(*exec.ExitError); ok {
			os.Exit(exiterr.ExitCode())
		} else {
			panic(err)
		}
	}
}
