//go:build windows

package filemerge

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	fileDeleteChild       windows.ACCESS_MASK = 0x00000040
	atomicDirectoryRights                     = windows.FILE_WRITE_DATA

	accessDeniedObjectACEType         = 6
	accessDeniedCallbackACEType       = 10
	accessDeniedCallbackObjectACEType = 12
	accessDeniedACEPrefixSize         = 8
	accessDeniedObjectFlagsSize       = 4
	guidSize                          = 16
	minimumSIDSize                    = 8
)

var errPresentNullDACL = errors.New("security descriptor has a present NULL DACL")

type relaxDirectoryACL func(string) (func() error, error)

type setDirectoryDACL func(string, windows.SECURITY_INFORMATION, *windows.ACL) error

func createAtomicTemp(dir, _ string) (*os.File, func() error, error) {
	tmp, err := os.CreateTemp(dir, ".gentle-ai-*.tmp")
	return tmp, nil, err
}

func createAtomicTempWith(dir, target string, createTemp func(string, string) (*os.File, error)) (*os.File, func() error, error) {
	return createAtomicTempWithACL(dir, target, createTemp, relaxAtomicDirectoryACL)
}

func createAtomicTempWithACL(dir, target string, createTemp func(string, string) (*os.File, error), relax relaxDirectoryACL) (*os.File, func() error, error) {
	tmp, err := createTemp(dir, ".gentle-ai-*.tmp")
	if err == nil || !errors.Is(err, fs.ErrPermission) {
		return tmp, nil, err
	}
	initialErr := err
	denied, err := targetDeniesDelete(target)
	if err != nil {
		return nil, nil, fmt.Errorf("inspect destination DACL %q: %w", target, err)
	}
	if denied {
		return nil, nil, fmt.Errorf("existing destination DACL denies delete: %w", fs.ErrPermission)
	}
	restore, err := relax(dir)
	if err != nil {
		if errors.Is(err, errPresentNullDACL) {
			return nil, nil, initialErr
		}
		return nil, nil, fmt.Errorf("relax parent directory ACL %q: %w", dir, err)
	}
	tmp, err = createTemp(dir, ".gentle-ai-*.tmp")
	if err != nil {
		retryErr := fmt.Errorf("retry after relaxing parent directory ACL: %w", err)
		if restoreErr := restore(); restoreErr != nil {
			restoreRetryErr := restore()
			return nil, nil, errors.Join(retryErr, fmt.Errorf("restore parent directory ACL: %w", errors.Join(restoreErr, restoreRetryErr)))
		}
		return nil, nil, retryErr
	}
	return tmp, restore, nil
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

func relaxAtomicDirectoryACL(dir string) (func() error, error) {
	return relaxAtomicDirectoryACLWith(dir, func(path string, securityInfo windows.SECURITY_INFORMATION, dacl *windows.ACL) error {
		return windows.SetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, securityInfo, nil, nil, dacl, nil)
	})
}

func relaxAtomicDirectoryACLWith(dir string, setDACL setDirectoryDACL) (func() error, error) {
	descriptor, err := windows.GetNamedSecurityInfo(
		dir,
		windows.SE_FILE_OBJECT,
		windows.DACL_SECURITY_INFORMATION,
	)
	if err != nil {
		return nil, fmt.Errorf("read DACL: %w", err)
	}
	dacl, _, err := descriptor.DACL()
	if err != nil {
		return nil, fmt.Errorf("read DACL entries: %w", err)
	}
	if dacl == nil {
		return nil, errPresentNullDACL
	}
	denied, err := deniesAccessRights(dacl, atomicDirectoryRights)
	if err != nil {
		return nil, fmt.Errorf("inspect DACL: %w", err)
	}
	if denied {
		return nil, fmt.Errorf("existing DACL denies atomic directory rights: %w", fs.ErrPermission)
	}
	control, _, err := descriptor.Control()
	if err != nil {
		return nil, fmt.Errorf("read DACL control: %w", err)
	}
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return nil, fmt.Errorf("read process user: %w", err)
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
		return nil, fmt.Errorf("merge DACL: %w", err)
	}
	securityInfo := windows.SECURITY_INFORMATION(windows.DACL_SECURITY_INFORMATION | windows.UNPROTECTED_DACL_SECURITY_INFORMATION)
	if control&windows.SE_DACL_PROTECTED != 0 {
		securityInfo = windows.SECURITY_INFORMATION(windows.DACL_SECURITY_INFORMATION | windows.PROTECTED_DACL_SECURITY_INFORMATION)
	}
	if err := setDACL(dir, securityInfo, acl); err != nil {
		return nil, fmt.Errorf("write DACL: %w", err)
	}
	return func() error {
		if err := setDACL(dir, securityInfo, dacl); err != nil {
			return fmt.Errorf("write original DACL: %w", err)
		}
		return nil
	}, nil
}

func deniesAccessRights(dacl *windows.ACL, rights windows.ACCESS_MASK) (bool, error) {
	if dacl == nil {
		return false, nil
	}
	token := windows.GetCurrentProcessToken()
	for i := uint32(0); i < uint32(dacl.AceCount); i++ {
		var ace *windows.ACCESS_ALLOWED_ACE
		if err := windows.GetAce(dacl, i, &ace); err != nil {
			return false, err
		}
		if !isAccessDeniedACEType(ace.Header.AceType) || ace.Header.AceFlags&windows.INHERIT_ONLY_ACE != 0 {
			continue
		}
		if ace.Header.AceSize < accessDeniedACEPrefixSize {
			return false, fmt.Errorf("access-denied ACE %d is too small: %d bytes", i, ace.Header.AceSize)
		}
		if ace.Mask&rights == 0 {
			continue
		}
		sid, err := accessDeniedACESID(unsafe.Pointer(ace), ace.Header)
		if err != nil {
			return false, fmt.Errorf("parse access-denied ACE %d: %w", i, err)
		}
		applies, err := denySIDApplies(token, sid)
		if err != nil {
			return false, fmt.Errorf("check access-denied ACE %d trustee: %w", i, err)
		}
		if applies {
			return true, nil
		}
	}
	return false, nil
}

func denySIDApplies(token windows.Token, sid *windows.SID) (bool, error) {
	user, err := token.GetTokenUser()
	if err != nil {
		return false, err
	}
	if user.User.Sid.Equals(sid) {
		return true, nil
	}
	groups, err := token.GetTokenGroups()
	if err != nil {
		return false, err
	}
	for _, group := range groups.AllGroups() {
		if group.Sid.Equals(sid) && denyGroupApplies(group.Attributes) {
			return true, nil
		}
	}
	return false, nil
}

func denyGroupApplies(attributes uint32) bool {
	return attributes&(windows.SE_GROUP_ENABLED|windows.SE_GROUP_USE_FOR_DENY_ONLY) != 0
}

func accessDeniedACESID(ace unsafe.Pointer, header windows.ACE_HEADER) (*windows.SID, error) {
	sidOffset := accessDeniedACEPrefixSize
	if header.AceType == accessDeniedObjectACEType || header.AceType == accessDeniedCallbackObjectACEType {
		if header.AceSize < accessDeniedACEPrefixSize+accessDeniedObjectFlagsSize {
			return nil, fmt.Errorf("object ACE is too small: %d bytes", header.AceSize)
		}
		flags := *(*uint32)(unsafe.Add(ace, accessDeniedACEPrefixSize))
		sidOffset += accessDeniedObjectFlagsSize
		if flags&windows.ACE_OBJECT_TYPE_PRESENT != 0 {
			sidOffset += guidSize
		}
		if flags&windows.ACE_INHERITED_OBJECT_TYPE_PRESENT != 0 {
			sidOffset += guidSize
		}
	}

	aceSize := int(header.AceSize)
	if sidOffset > aceSize-minimumSIDSize {
		return nil, fmt.Errorf("ACE size %d does not contain a SID at offset %d", aceSize, sidOffset)
	}
	sidBytes := unsafe.Slice((*byte)(unsafe.Add(ace, sidOffset)), aceSize-sidOffset)
	sidSize := minimumSIDSize + 4*int(sidBytes[1])
	if sidSize > len(sidBytes) {
		return nil, fmt.Errorf("SID size %d exceeds remaining ACE size %d", sidSize, len(sidBytes))
	}
	sid := (*windows.SID)(unsafe.Add(ace, sidOffset))
	if !sid.IsValid() {
		return nil, errors.New("ACE contains an invalid SID")
	}
	return sid, nil
}

func isAccessDeniedACEType(aceType uint8) bool {
	// Standard, object, callback, and callback-object access-denied ACEs.
	return aceType == windows.ACCESS_DENIED_ACE_TYPE || aceType == accessDeniedObjectACEType || aceType == accessDeniedCallbackACEType || aceType == accessDeniedCallbackObjectACEType
}
