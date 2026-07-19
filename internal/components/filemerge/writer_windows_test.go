//go:build windows

package filemerge

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/sys/windows"
)

func TestDenyGroupApplies(t *testing.T) {
	tests := []struct {
		name       string
		attributes uint32
		want       bool
	}{
		{"enabled", windows.SE_GROUP_ENABLED, true},
		{"deny-only", windows.SE_GROUP_USE_FOR_DENY_ONLY, true},
		{"disabled despite default", windows.SE_GROUP_ENABLED_BY_DEFAULT, false},
		{"disabled", 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := denyGroupApplies(tt.attributes); got != tt.want {
				t.Fatalf("denyGroupApplies(%#x) = %t, want %t", tt.attributes, got, tt.want)
			}
		})
	}
}

func TestWriteFileAtomicRelaxesWindowsDirectoryACL(t *testing.T) {
	dir := t.TempDir()
	requirePersistentACL(t, dir)
	restoreDACL(t, dir)

	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		t.Fatalf("GetTokenUser() error = %v", err)
	}
	readOnlyACL, err := windows.ACLFromEntries([]windows.EXPLICIT_ACCESS{{
		AccessPermissions: windows.GENERIC_READ | windows.GENERIC_EXECUTE,
		AccessMode:        windows.GRANT_ACCESS,
		Trustee: windows.TRUSTEE{
			TrusteeForm:  windows.TRUSTEE_IS_SID,
			TrusteeType:  windows.TRUSTEE_IS_USER,
			TrusteeValue: windows.TrusteeValueFromSID(user.User.Sid),
		},
	}}, nil)
	if err != nil {
		t.Fatalf("ACLFromEntries() error = %v", err)
	}
	if err := windows.SetNamedSecurityInfo(
		dir,
		windows.SE_FILE_OBJECT,
		windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION,
		nil,
		nil,
		readOnlyACL,
		nil,
	); err != nil {
		t.Fatalf("SetNamedSecurityInfo(read-only fixture) error = %v", err)
	}

	probe, err := os.CreateTemp(dir, "acl-probe-*.tmp")
	if err == nil {
		probe.Close()
		os.Remove(probe.Name())
		t.Fatal("os.CreateTemp() succeeded before WriteFileAtomic; fixture did not deny file creation")
	}
	if !errors.Is(err, fs.ErrPermission) {
		t.Fatalf("os.CreateTemp() error = %v, want fs.ErrPermission", err)
	}

	path := filepath.Join(dir, "SKILL.md")
	content := []byte("# Windows ACL\n")
	if _, err := WriteFileAtomic(path, content, 0o644); err != nil {
		t.Fatalf("WriteFileAtomic() error = %v, want successful ACL relaxation", err)
	}
	descriptor, err := windows.GetNamedSecurityInfo(dir, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		t.Fatalf("GetNamedSecurityInfo(after write) error = %v", err)
	}
	control, _, err := descriptor.Control()
	if err != nil {
		t.Fatalf("SECURITY_DESCRIPTOR.Control(after write) error = %v", err)
	}
	if control&windows.SE_DACL_PROTECTED == 0 {
		t.Fatal("directory DACL became unprotected after ACL relaxation")
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if string(got) != string(content) {
		t.Fatalf("file content = %q, want %q", got, content)
	}
}

func TestWriteFileAtomicDoesNotBypassExplicitWindowsDeny(t *testing.T) {
	dir := t.TempDir()
	requirePersistentACL(t, dir)
	restoreDACL(t, dir)

	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		t.Fatalf("GetTokenUser() error = %v", err)
	}
	trustee := windows.TRUSTEE{
		TrusteeForm:  windows.TRUSTEE_IS_SID,
		TrusteeType:  windows.TRUSTEE_IS_USER,
		TrusteeValue: windows.TrusteeValueFromSID(user.User.Sid),
	}
	dacl, err := windows.ACLFromEntries([]windows.EXPLICIT_ACCESS{
		{
			AccessPermissions: windows.FILE_WRITE_DATA,
			AccessMode:        windows.DENY_ACCESS,
			Trustee:           trustee,
		},
		{
			AccessPermissions: windows.GENERIC_READ | windows.GENERIC_EXECUTE,
			AccessMode:        windows.GRANT_ACCESS,
			Trustee:           trustee,
		},
	}, nil)
	if err != nil {
		t.Fatalf("ACLFromEntries() error = %v", err)
	}
	if err := windows.SetNamedSecurityInfo(
		dir,
		windows.SE_FILE_OBJECT,
		windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION,
		nil,
		nil,
		dacl,
		nil,
	); err != nil {
		t.Fatalf("SetNamedSecurityInfo(explicit-deny fixture) error = %v", err)
	}

	path := filepath.Join(dir, "blocked.txt")
	_, err = WriteFileAtomic(path, []byte("blocked\n"), 0o644)
	if !errors.Is(err, fs.ErrPermission) {
		t.Fatalf("WriteFileAtomic() error = %v, want fs.ErrPermission", err)
	}
	if _, statErr := os.Stat(path); !errors.Is(statErr, fs.ErrNotExist) {
		t.Fatalf("Stat(blocked target) error = %v, want fs.ErrNotExist", statErr)
	}
}

func TestWriteFileAtomicIgnoresUnrelatedWindowsDeny(t *testing.T) {
	dir := t.TempDir()
	requirePersistentACL(t, dir)
	restoreDACL(t, dir)

	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		t.Fatalf("GetTokenUser() error = %v", err)
	}
	unrelated, err := windows.StringToSid("S-1-5-21-1111111111-2222222222-3333333333-1001")
	if err != nil {
		t.Fatalf("StringToSid() error = %v", err)
	}
	member, err := windows.GetCurrentProcessToken().IsMember(unrelated)
	if err != nil {
		t.Fatalf("IsMember(unrelated SID) error = %v", err)
	}
	if member {
		t.Fatal("unrelated SID unexpectedly applies to the current process token")
	}

	dacl, err := windows.ACLFromEntries([]windows.EXPLICIT_ACCESS{
		{
			AccessPermissions: windows.FILE_WRITE_DATA,
			AccessMode:        windows.DENY_ACCESS,
			Trustee: windows.TRUSTEE{
				TrusteeForm:  windows.TRUSTEE_IS_SID,
				TrusteeType:  windows.TRUSTEE_IS_USER,
				TrusteeValue: windows.TrusteeValueFromSID(unrelated),
			},
		},
		{
			AccessPermissions: windows.GENERIC_READ | windows.GENERIC_EXECUTE,
			AccessMode:        windows.GRANT_ACCESS,
			Trustee: windows.TRUSTEE{
				TrusteeForm:  windows.TRUSTEE_IS_SID,
				TrusteeType:  windows.TRUSTEE_IS_USER,
				TrusteeValue: windows.TrusteeValueFromSID(user.User.Sid),
			},
		},
	}, nil)
	if err != nil {
		t.Fatalf("ACLFromEntries() error = %v", err)
	}
	if err := windows.SetNamedSecurityInfo(
		dir,
		windows.SE_FILE_OBJECT,
		windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION,
		nil,
		nil,
		dacl,
		nil,
	); err != nil {
		t.Fatalf("SetNamedSecurityInfo(unrelated-deny fixture) error = %v", err)
	}

	path := filepath.Join(dir, "allowed.txt")
	content := []byte("allowed\n")
	if _, err := WriteFileAtomic(path, content, 0o644); err != nil {
		t.Fatalf("WriteFileAtomic() error = %v, want unrelated deny ignored", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if string(got) != string(content) {
		t.Fatalf("file content = %q, want %q", got, content)
	}
}

func TestWriteFileAtomicDoesNotReplaceTargetWithExplicitDeleteDeny(t *testing.T) {
	dir := t.TempDir()
	requirePersistentACL(t, dir)
	restoreDACL(t, dir)
	path := filepath.Join(dir, "protected.txt")
	original := []byte("original\n")
	if err := os.WriteFile(path, original, 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	restoreDACL(t, path)

	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		t.Fatalf("GetTokenUser() error = %v", err)
	}
	trustee := windows.TRUSTEE{TrusteeForm: windows.TRUSTEE_IS_SID, TrusteeType: windows.TRUSTEE_IS_USER, TrusteeValue: windows.TrusteeValueFromSID(user.User.Sid)}
	targetACL, err := windows.ACLFromEntries([]windows.EXPLICIT_ACCESS{{AccessPermissions: windows.DELETE, AccessMode: windows.DENY_ACCESS, Trustee: trustee}, {AccessPermissions: windows.GENERIC_READ | windows.GENERIC_WRITE, AccessMode: windows.GRANT_ACCESS, Trustee: trustee}}, nil)
	if err != nil {
		t.Fatalf("ACLFromEntries(target) error = %v", err)
	}
	if err := windows.SetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION, nil, nil, targetACL, nil); err != nil {
		t.Fatalf("SetNamedSecurityInfo(target) error = %v", err)
	}
	parentACL, err := windows.ACLFromEntries([]windows.EXPLICIT_ACCESS{{AccessPermissions: windows.GENERIC_READ | windows.GENERIC_EXECUTE, AccessMode: windows.GRANT_ACCESS, Trustee: trustee}}, nil)
	if err != nil {
		t.Fatalf("ACLFromEntries(parent) error = %v", err)
	}
	if err := windows.SetNamedSecurityInfo(dir, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION, nil, nil, parentACL, nil); err != nil {
		t.Fatalf("SetNamedSecurityInfo(parent) error = %v", err)
	}

	_, err = WriteFileAtomic(path, []byte("replacement\n"), 0o644)
	if !errors.Is(err, fs.ErrPermission) {
		t.Fatalf("WriteFileAtomic() error = %v, want fs.ErrPermission", err)
	}
	got, readErr := os.ReadFile(path)
	if readErr != nil {
		t.Fatalf("ReadFile() error = %v", readErr)
	}
	if string(got) != string(original) {
		t.Fatalf("file content = %q, want unchanged %q", got, original)
	}
	if probe, createErr := os.CreateTemp(dir, "acl-probe-*.tmp"); createErr == nil {
		probe.Close()
		os.Remove(probe.Name())
		t.Fatal("os.CreateTemp() succeeded after denied write; parent DACL was relaxed")
	} else if !errors.Is(createErr, fs.ErrPermission) {
		t.Fatalf("os.CreateTemp() error = %v, want fs.ErrPermission", createErr)
	}
}

func requirePersistentACL(t *testing.T, path string) {
	t.Helper()
	pathPtr, err := windows.UTF16PtrFromString(path)
	if err != nil {
		t.Fatalf("UTF16PtrFromString() error = %v", err)
	}
	volumePath := make([]uint16, windows.MAX_PATH+1)
	if err := windows.GetVolumePathName(pathPtr, &volumePath[0], uint32(len(volumePath))); err != nil {
		t.Fatalf("GetVolumePathName() error = %v", err)
	}
	var flags uint32
	if err := windows.GetVolumeInformation(
		&volumePath[0], nil, 0, nil, nil, &flags, nil, 0,
	); err != nil {
		t.Fatalf("GetVolumeInformation() error = %v", err)
	}
	if flags&windows.FILE_PERSISTENT_ACLS == 0 {
		t.Skipf("volume %q does not support persistent ACLs", windows.UTF16ToString(volumePath))
	}
}

func restoreDACL(t *testing.T, path string) {
	t.Helper()
	descriptor, err := windows.GetNamedSecurityInfo(
		path,
		windows.SE_FILE_OBJECT,
		windows.DACL_SECURITY_INFORMATION,
	)
	if err != nil {
		t.Fatalf("GetNamedSecurityInfo() error = %v", err)
	}
	dacl, _, err := descriptor.DACL()
	if err != nil {
		t.Fatalf("SECURITY_DESCRIPTOR.DACL() error = %v", err)
	}
	control, _, err := descriptor.Control()
	if err != nil {
		t.Fatalf("SECURITY_DESCRIPTOR.Control() error = %v", err)
	}
	securityInfo := windows.SECURITY_INFORMATION(windows.DACL_SECURITY_INFORMATION | windows.UNPROTECTED_DACL_SECURITY_INFORMATION)
	if control&windows.SE_DACL_PROTECTED != 0 {
		securityInfo = windows.SECURITY_INFORMATION(windows.DACL_SECURITY_INFORMATION | windows.PROTECTED_DACL_SECURITY_INFORMATION)
	}
	t.Cleanup(func() {
		if err := windows.SetNamedSecurityInfo(
			path,
			windows.SE_FILE_OBJECT,
			securityInfo,
			nil,
			nil,
			dacl,
			nil,
		); err != nil {
			t.Errorf("restore DACL: %v", err)
		}
	})
}
