//go:build windows

package main

import (
	"bytes"
	"crypto/sha256"
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"unsafe"

	"github.com/egoist/mygo"
	"golang.org/x/sys/windows"
)

//go:embed design/app-icon.ico
var taskbarIconICO []byte

type taskbarPropertyKey struct {
	Format windows.GUID
	ID     uint32
}
type taskbarVariant struct {
	Kind        uint16
	Reserved    [3]uint16
	Data, Extra uintptr
}

type taskbarStore struct{ table *[8]uintptr }

func taskbarStoreCall(store *taskbarStore, method int, args ...uintptr) uintptr {
	hr, _, _ := syscall.SyscallN(store.table[method], append([]uintptr{uintptr(unsafe.Pointer(store))}, args...)...)
	return hr
}

func configureTaskbarIcon(w *mygo.Window, profileDir string) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	root, err := os.UserCacheDir()
	if err != nil {
		return err
	}
	dir := filepath.Join(root, "Sidelet", "icons")
	if err = os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	hash := sha256.Sum256(taskbarIconICO)
	icon := filepath.Join(dir, fmt.Sprintf("note-%x.ico", hash[:8]))
	data, err := os.ReadFile(icon)
	if err != nil || !bytes.Equal(data, taskbarIconICO) {
		if err = os.WriteFile(icon, taskbarIconICO, 0600); err != nil {
			return err
		}
	}
	command := syscall.EscapeArg(exe) + " -main"
	if profileDir != "" {
		command += " -data-dir " + syscall.EscapeArg(profileDir)
	}
	// A stable identity per executable keeps installed and development builds
	// separate. Icon revisions never change the identity or pin target.
	identity := sha256.Sum256([]byte(strings.ToLower(filepath.Clean(exe))))
	return setTaskbarProperties(w.NativeHandle(), fmt.Sprintf("Sidelet.Desktop.%x", identity[:12]), command, icon+",0")
}

func setTaskbarProperties(hwnd uintptr, id, command, icon string) error {
	iid, _ := windows.GUIDFromString("{886D8EEB-8CF2-4446-8D02-CDBA1DBDCF99}")
	format, _ := windows.GUIDFromString("{9F4C2855-9F79-4B39-A8D0-E1D42DE1D5F3}")
	var store *taskbarStore
	hr, _, _ := syscall.NewLazyDLL("shell32.dll").NewProc("SHGetPropertyStoreForWindow").Call(hwnd, uintptr(unsafe.Pointer(&iid)), uintptr(unsafe.Pointer(&store)))
	if int32(hr) < 0 {
		return fmt.Errorf("taskbar property store: %#x", uint32(hr))
	}
	defer taskbarStoreCall(store, 2)
	// Relaunch metadata must precede the explicit AppUserModelID. The shell
	// uses these properties for the application row and for new pinned links.
	for _, entry := range []struct {
		key   uint32
		value string
	}{{2, command}, {4, "Sidelet"}, {3, icon}, {5, id}} {
		text, err := windows.UTF16FromString(entry.value)
		if err != nil {
			return err
		}
		key := taskbarPropertyKey{format, entry.key}
		value := taskbarVariant{Kind: 31, Data: uintptr(unsafe.Pointer(&text[0]))}
		hr = taskbarStoreCall(store, 6, uintptr(unsafe.Pointer(&key)), uintptr(unsafe.Pointer(&value)))
		runtime.KeepAlive(text)
		if int32(hr) < 0 {
			return fmt.Errorf("taskbar property %d: %#x", entry.key, uint32(hr))
		}
	}
	return nil
}
