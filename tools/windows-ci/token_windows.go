package main

// This helper belongs to disposable CI only. It is not included in Sidelet.
import (
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

type tokenState struct {
	PID             uint32                      `json:"pid"`
	Elevated        bool                        `json:"elevated"`
	Integrity       string                      `json:"integrity"`
	AdminAttributes uint32                      `json:"adminAttributes"`
	Privileges      []windows.LUIDAndAttributes `json:"privileges"`
	OtherEnabled    bool                        `json:"otherEnabledPrivileges"`
}

func tokenInfo(token windows.Token, class uint32) ([]byte, error) {
	// Variable-size classes only. Fixed-size TokenElevation requires its exact
	// structure size on Windows, even when a larger buffer would fit.
	for size := uint32(128); ; {
		buf := make([]byte, size)
		err := windows.GetTokenInformation(token, class, &buf[0], size, &size)
		if err == nil {
			return buf, nil
		}
		if (err != windows.ERROR_INSUFFICIENT_BUFFER && err != windows.ERROR_BAD_LENGTH) || size <= uint32(len(buf)) {
			return nil, fmt.Errorf("token information %d: %w", class, err)
		}
	}
}

func inspectToken(token windows.Token) (tokenState, error) {
	var state tokenState
	var elevation, returned uint32
	if err := windows.GetTokenInformation(token, windows.TokenElevation, (*byte)(unsafe.Pointer(&elevation)), uint32(unsafe.Sizeof(elevation)), &returned); err != nil {
		return state, err
	}
	if returned != uint32(unsafe.Sizeof(elevation)) {
		return state, fmt.Errorf("unexpected elevation information length: %d", returned)
	}
	state.Elevated = elevation != 0
	label, err := tokenInfo(token, windows.TokenIntegrityLevel)
	if err != nil {
		return state, err
	}
	state.Integrity = (*windows.Tokenmandatorylabel)(unsafe.Pointer(&label[0])).Label.Sid.String()
	runtime.KeepAlive(label)
	groups, err := token.GetTokenGroups()
	if err != nil {
		return state, err
	}
	for _, group := range groups.AllGroups() {
		if group.Sid.IsWellKnown(windows.WinBuiltinAdministratorsSid) {
			state.AdminAttributes = group.Attributes
		}
	}
	privileges, err := tokenInfo(token, windows.TokenPrivileges)
	if err != nil {
		return state, err
	}
	var traverse windows.LUID
	name, _ := windows.UTF16PtrFromString("SeChangeNotifyPrivilege")
	if err := windows.LookupPrivilegeValue(nil, name, &traverse); err != nil {
		return state, err
	}
	state.Privileges = append([]windows.LUIDAndAttributes{}, (*windows.Tokenprivileges)(unsafe.Pointer(&privileges[0])).AllPrivileges()...)
	for _, privilege := range state.Privileges {
		if privilege.Luid != traverse && privilege.Attributes&windows.SE_PRIVILEGE_ENABLED != 0 {
			state.OtherEnabled = true
		}
	}
	return state, nil
}

func (state tokenState) validate() error {
	if state.Elevated || state.Integrity != "S-1-16-8192" || state.OtherEnabled ||
		state.AdminAttributes&windows.SE_GROUP_ENABLED != 0 ||
		(state.AdminAttributes != 0 && state.AdminAttributes&windows.SE_GROUP_USE_FOR_DENY_ONLY == 0) {
		return fmt.Errorf("expected non-elevated medium-integrity token without admin access: %+v", state)
	}
	return nil
}

func checkMachineWriteDenied(report string) error {
	state, err := inspectToken(windows.GetCurrentProcessToken())
	if err != nil {
		return err
	}
	if err := state.validate(); err != nil {
		return err
	}
	// Open an existing key requesting write access. Never create or modify it.
	key, err := registry.OpenKey(registry.LOCAL_MACHINE, `SOFTWARE`, registry.WRITE|registry.WOW64_64KEY)
	if err == nil {
		key.Close()
		return fmt.Errorf("restricted process unexpectedly has machine-wide software registry write access")
	}
	if err != windows.ERROR_ACCESS_DENIED {
		return fmt.Errorf("expected ERROR_ACCESS_DENIED opening machine software key, got: %w", err)
	}
	return os.WriteFile(report, []byte("{\"machineSoftwareWriteAccess\":\"denied\",\"modifiedSystemRegistry\":false}\n"), 0600)
}

func recordProcess(pid uint32, report string) error {
	process, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, pid)
	if err != nil {
		return err
	}
	defer windows.CloseHandle(process)
	var token windows.Token
	if err := windows.OpenProcessToken(process, windows.TOKEN_QUERY, &token); err != nil {
		return err
	}
	defer token.Close()
	state, err := inspectToken(token)
	if err != nil {
		return err
	}
	state.PID = pid
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(report, append(data, '\n'), 0600); err != nil {
		return err
	}
	return state.validate()
}

func restrictedToken() (windows.Token, error) {
	var original windows.Token
	if err := windows.OpenProcessToken(windows.CurrentProcess(), windows.TOKEN_QUERY|windows.TOKEN_DUPLICATE|windows.TOKEN_ASSIGN_PRIMARY|windows.TOKEN_ADJUST_DEFAULT, &original); err != nil {
		return 0, err
	}
	defer original.Close()
	admin, err := windows.CreateWellKnownSid(windows.WinBuiltinAdministratorsSid)
	if err != nil {
		return 0, err
	}
	disabled := windows.SIDAndAttributes{Sid: admin}
	var token windows.Token
	// LUA_TOKEN filters administrator groups; DISABLE_MAX_PRIVILEGE leaves
	// only traverse checking enabled. No UAC policy or account changes.
	create := windows.NewLazySystemDLL("advapi32.dll").NewProc("CreateRestrictedToken")
	ok, _, callErr := create.Call(uintptr(original), 0x1|0x4, 1, uintptr(unsafe.Pointer(&disabled)), 0, 0, 0, 0, uintptr(unsafe.Pointer(&token)))
	runtime.KeepAlive(disabled)
	if ok == 0 {
		return 0, fmt.Errorf("CreateRestrictedToken: %w", callErr)
	}
	medium, err := windows.StringToSid("S-1-16-8192")
	if err == nil {
		label := windows.Tokenmandatorylabel{Label: windows.SIDAndAttributes{Sid: medium, Attributes: windows.SE_GROUP_INTEGRITY}}
		err = windows.SetTokenInformation(token, windows.TokenIntegrityLevel, (*byte)(unsafe.Pointer(&label)), label.Size())
		runtime.KeepAlive(medium)
	}
	if err == nil {
		var state tokenState
		state, err = inspectToken(token)
		if err == nil {
			err = state.validate()
		}
	}
	if err != nil {
		token.Close()
		return 0, err
	}
	return token, nil
}
