package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

func main() {
	code, err := run(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	os.Exit(code)
}

func run(args []string) (int, error) {
	if os.Getenv("GITHUB_ACTIONS") != "true" || os.Getenv("RUNNER_ENVIRONMENT") != "github-hosted" {
		return 0, fmt.Errorf("Windows permission checks require a disposable GitHub-hosted runner")
	}
	if len(args) == 3 && args[0] == "check" {
		pid, err := strconv.ParseUint(args[1], 10, 32)
		if err != nil {
			return 0, err
		}
		return 0, recordProcess(uint32(pid), args[2])
	}
	if len(args) >= 3 && args[0] == "setup" {
		return runSetup(args[1], args[2:])
	}
	if len(args) != 3 || args[0] != "suite" || args[1] != runtime.GOARCH {
		return 0, fmt.Errorf("usage: windows-ci suite <native arch> <repo> | check <pid> <report> | setup <report> <exe> [args...]")
	}
	root, err := filepath.Abs(args[2])
	if err != nil {
		return 0, err
	}
	self, err := os.Executable()
	if err != nil {
		return 0, err
	}
	token, err := restrictedToken()
	if err != nil {
		return 0, err
	}
	defer token.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "pwsh", "-NoLogo", "-NoProfile", "-NonInteractive", "-File", filepath.Join(root, "scripts", "test-windows-nonadmin.ps1"), "-Arch", args[1], "-TokenProbe", self)
	cmd.Dir = root
	cmd.SysProcAttr = &syscall.SysProcAttr{Token: syscall.Token(token)}
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	cmd.WaitDelay = 5 * time.Second
	return 0, cmd.Run()
}

// Setup processes can exit too quickly for a post-launch PID probe. Start the
// exact EXE suspended, inspect its actual token, then allow it to run. This uses
// the caller's existing token; there is no shell/elevation broker in this path.
func runSetup(report string, args []string) (int, error) {
	state, err := inspectToken(windows.GetCurrentProcessToken())
	if err != nil {
		return 0, err
	}
	if err := state.validate(); err != nil {
		return 0, err
	}
	path, err := filepath.Abs(args[0])
	if err != nil {
		return 0, err
	}
	args[0] = path
	app, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return 0, err
	}
	command, err := windows.UTF16PtrFromString(windows.ComposeCommandLine(args))
	if err != nil {
		return 0, err
	}
	startup := windows.StartupInfo{Cb: uint32(unsafe.Sizeof(windows.StartupInfo{}))}
	var process windows.ProcessInformation
	if err := windows.CreateProcess(app, command, nil, nil, false, windows.CREATE_SUSPENDED, nil, nil, &startup, &process); err != nil {
		return 0, err
	}
	defer windows.CloseHandle(process.Process)
	defer windows.CloseHandle(process.Thread)
	finished := false
	defer func() {
		if !finished {
			windows.TerminateProcess(process.Process, 1)
			windows.WaitForSingleObject(process.Process, 5000)
		}
	}()
	if err := recordProcess(process.ProcessId, report); err != nil {
		return 0, err
	}
	if _, err := windows.ResumeThread(process.Thread); err != nil {
		return 0, err
	}
	result, err := windows.WaitForSingleObject(process.Process, 60000)
	if err != nil {
		return 0, err
	}
	if result != windows.WAIT_OBJECT_0 {
		return 0, fmt.Errorf("setup timed out: %s", path)
	}
	finished = true
	var code uint32
	if err := windows.GetExitCodeProcess(process.Process, &code); err != nil {
		return 0, err
	}
	return int(code), nil
}
