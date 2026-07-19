//go:build windows

package filemerge

import (
	"errors"
	"fmt"
	"io/fs"
	"os"

	"golang.org/x/sys/windows"
)

const (
	fileDeleteChild       windows.ACCESS_MASK = 0x00000040
	atomicDirectoryRights                     = windows.FILE_WRITE_DATA | fileDeleteChild
)

func createAtomicTemp(dir, target string) (*os.File, error) {
	tmp, err := os.CreateTemp(dir, ".gentle-ai-*.tmp")
	if err == nil || !errors.Is(err, fs.ErrPermission) {
		return tmp, err
	}
	denied, err := targetDeniesDelete(target)
	if err != nil {
		return nil, fmt.Errorf("inspect destination DACL %q: %w", target, err)
	}
	if denied {
		return nil, fmt.Errorf("existing destination DACL denies delete: %w", fs.ErrPermission)
	}
	if err := relaxAtomicDirectoryACL(dir); err != nil {
		return nil, fmt.Errorf("relax parent directory ACL %q: %w", dir, err)
	}
	tmp, err = os.CreateTemp(dir, ".gentle-ai-*.tmp")
	if err != nil {
		return nil, fmt.Errorf("retry after relaxing parent directory ACL: %w", err)
	}
	return tmp, nil
}

func targetDeniesDelete(path string) (bool, error) {
	descriptor, err := windows.GetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
	if errors.Is(err, windows.ERROR_FILE_NOT_FOUND) || errors.Is(err, windows.ERROR_PATH_NOT_FOUND) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	dacl, _, err := descriptor.DACL()
	if err != nil {
		return false, err
	}
	return deniesAccessRights(dacl, windows.DELETE)
}

func relaxAtomicDirectoryACL(dir string) error {
	descriptor, err := windows.GetNamedSecurityInfo(
		dir,
		windows.SE_FILE_OBJECT,
		windows.DACL_SECURITY_INFORMATION,
	)
	if err != nil {
		return fmt.Errorf("read DACL: %w", err)
	}
	dacl, _, err := descriptor.DACL()
	if err != nil {
		return fmt.Errorf("read DACL entries: %w", err)
	}
	denied, err := deniesAccessRights(dacl, atomicDirectoryRights)
	if err != nil {
		return fmt.Errorf("inspect DACL: %w", err)
	}
	if denied {
		return fmt.Errorf("existing DACL denies atomic directory rights: %w", fs.ErrPermission)
	}
	control, _, err := descriptor.Control()
	if err != nil {
		return fmt.Errorf("read DACL control: %w", err)
	}
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return fmt.Errorf("read process user: %w", err)
	}

	// FILE_WRITE_DATA is FILE_ADD_FILE when the secured object is a directory.
	acl, err := windows.ACLFromEntries([]windows.EXPLICIT_ACCESS{{
		AccessPermissions: atomicDirectoryRights,
		AccessMode:        windows.GRANT_ACCESS,
		Inheritance:       windows.NO_INHERITANCE,
		Trustee: windows.TRUSTEE{
			TrusteeForm:  windows.TRUSTEE_IS_SID,
			TrusteeType:  windows.TRUSTEE_IS_USER,
			TrusteeValue: windows.TrusteeValueFromSID(user.User.Sid),
		},
	}}, dacl)
	if err != nil {
		return fmt.Errorf("merge DACL: %w", err)
	}
	securityInfo := windows.SECURITY_INFORMATION(windows.DACL_SECURITY_INFORMATION | windows.UNPROTECTED_DACL_SECURITY_INFORMATION)
	if control&windows.SE_DACL_PROTECTED != 0 {
		securityInfo = windows.SECURITY_INFORMATION(windows.DACL_SECURITY_INFORMATION | windows.PROTECTED_DACL_SECURITY_INFORMATION)
	}
	if err := windows.SetNamedSecurityInfo(
		dir,
		windows.SE_FILE_OBJECT,
		securityInfo,
		nil,
		nil,
		acl,
		nil,
	); err != nil {
		return fmt.Errorf("write DACL: %w", err)
	}
	return nil
}

func deniesAccessRights(dacl *windows.ACL, rights windows.ACCESS_MASK) (bool, error) {
	if dacl == nil {
		return false, nil
	}
	for i := uint32(0); i < uint32(dacl.AceCount); i++ {
		var ace *windows.ACCESS_ALLOWED_ACE
		if err := windows.GetAce(dacl, i, &ace); err != nil {
			return false, err
		}
		if isAccessDeniedACEType(ace.Header.AceType) && ace.Mask&rights != 0 {
			return true, nil
		}
	}
	return false, nil
}

func isAccessDeniedACEType(aceType uint8) bool {
	// Standard, object, callback, and callback-object access-denied ACEs.
	return aceType == windows.ACCESS_DENIED_ACE_TYPE || aceType == 6 || aceType == 10 || aceType == 12
}
