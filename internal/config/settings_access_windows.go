//go:build windows

package config

import "golang.org/x/sys/windows"

func preserveSettingsFileAccess(source, destination string) error {
	sd, err := windows.GetNamedSecurityInfo(source, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
	if err != nil || sd == nil {
		return err
	}
	dacl, _, err := sd.DACL()
	if err != nil {
		return err
	}
	control, _, err := sd.Control()
	if err != nil {
		return err
	}
	securityInfo := windows.SECURITY_INFORMATION(windows.DACL_SECURITY_INFORMATION)
	if control&windows.SE_DACL_PROTECTED != 0 {
		securityInfo |= windows.PROTECTED_DACL_SECURITY_INFORMATION
	} else {
		securityInfo |= windows.UNPROTECTED_DACL_SECURITY_INFORMATION
	}
	return windows.SetNamedSecurityInfo(destination, windows.SE_FILE_OBJECT, securityInfo, nil, nil, dacl, nil)
}
