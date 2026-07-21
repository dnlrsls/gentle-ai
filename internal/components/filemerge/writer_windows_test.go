//go:build windows

package filemerge

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"unsafe"

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

func TestDeniesAccessRightsHandlesCallbackConditions(t *testing.T) {
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		t.Fatalf("GetTokenUser() error = %v", err)
	}

	tests := []struct {
		name            string
		aceType         uint8
		applicationData []byte
		wantDenied      bool
	}{
		{"plain callback deny", accessDeniedCallbackACEType, nil, true},
		{"conditional callback deny", accessDeniedCallbackACEType, []byte{'a', 'r', 't', 'x'}, true},
		{"plain callback-object deny", accessDeniedCallbackObjectACEType, nil, true},
		{"conditional callback-object deny", accessDeniedCallbackObjectACEType, []byte{'a', 'r', 't', 'x'}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dacl := callbackDenyACL(t, tt.aceType, windows.DELETE, user.User.Sid, tt.applicationData)
			got, err := deniesAccessRights(dacl, windows.DELETE)
			if err != nil {
				t.Fatalf("deniesAccessRights() error = %v", err)
			}
			if got != tt.wantDenied {
				t.Fatalf("deniesAccessRights() = %t, want %t", got, tt.wantDenied)
			}
		})
	}
}

func callbackDenyACL(t *testing.T, aceType uint8, mask windows.ACCESS_MASK, sid *windows.SID, applicationData []byte) *windows.ACL {
	t.Helper()
	const aclRevisionDS = 4
	sidOffset := accessDeniedACEPrefixSize
	if aceType == accessDeniedCallbackObjectACEType {
		sidOffset += accessDeniedObjectFlagsSize
	}
	aceSize := sidOffset + sid.Len() + len(applicationData)
	aclBytes := make([]byte, 8+aceSize)
	aclBytes[0] = aclRevisionDS
	binary.LittleEndian.PutUint16(aclBytes[2:4], uint16(len(aclBytes)))
	binary.LittleEndian.PutUint16(aclBytes[4:6], 1)
	aclBytes[8] = aceType
	binary.LittleEndian.PutUint16(aclBytes[10:12], uint16(aceSize))
	binary.LittleEndian.PutUint32(aclBytes[12:16], uint32(mask))
	copy(aclBytes[8+sidOffset:], unsafe.Slice((*byte)(unsafe.Pointer(sid)), sid.Len()))
	copy(aclBytes[8+sidOffset+sid.Len():], applicationData)
	return (*windows.ACL)(unsafe.Pointer(&aclBytes[0]))
}

func createAtomicTempWithRelaxedACL(dir, target string) (*os.File, func() error, error) {
	return createAtomicTempWith(dir, target, os.CreateTemp)
}

func TestWriteFileAtomicFailsClosedWhenWindowsDirectoryDeniesCreate(t *testing.T) {
	dir := t.TempDir()
	requirePersistentACL(t, dir)
	restoreDACL(t, dir)
	setReadOnlyDirectoryACL(t, dir, true)
	originalACL := captureDACL(t, dir)
	path := filepath.Join(dir, "SKILL.md")

	_, err := WriteFileAtomic(path, []byte("# Windows ACL\n"), 0o644)
	if !errors.Is(err, fs.ErrPermission) {
		t.Fatalf("WriteFileAtomic() error = %v, want fs.ErrPermission", err)
	}
	assertDACLMatches(t, dir, originalACL)
	if _, statErr := os.Stat(path); !errors.Is(statErr, fs.ErrNotExist) {
		t.Fatalf("Stat(target) error = %v, want fs.ErrNotExist", statErr)
	}
	tmpFiles, globErr := filepath.Glob(filepath.Join(dir, ".gentle-ai-*.tmp"))
	if globErr != nil {
		t.Fatalf("Glob(temp files) error = %v", globErr)
	}
	if len(tmpFiles) != 0 {
		t.Fatalf("temporary files = %v, want none", tmpFiles)
	}
}

func TestCreateAtomicTempRestoresWindowsDirectoryACLAfterRetryFailure(t *testing.T) {
	dir := t.TempDir()
	requirePersistentACL(t, dir)
	restoreDACL(t, dir)
	setReadOnlyDirectoryACL(t, dir, true)
	originalACL := captureDACL(t, dir)

	initialErr := &os.PathError{Op: "createtemp", Path: dir, Err: windows.ERROR_ACCESS_DENIED}
	retryErr := errors.New("retry failed")
	createCalls := 0
	_, _, err := createAtomicTempWith(dir, filepath.Join(dir, "target.txt"), func(string, string) (*os.File, error) {
		createCalls++
		if createCalls == 1 {
			return nil, initialErr
		}
		return nil, retryErr
	})
	if !errors.Is(err, retryErr) {
		t.Fatalf("createAtomicTempWith() error = %v, want retry error %v", err, retryErr)
	}
	if createCalls != 2 {
		t.Fatalf("CreateTemp calls = %d, want 2", createCalls)
	}
	assertDACLMatches(t, dir, originalACL)
}

func TestCreateAtomicTempRestoresExactWindowsDACLState(t *testing.T) {
	tests := []struct {
		name      string
		configure func(*testing.T, string)
	}{
		{
			name: "protected DACL",
			configure: func(t *testing.T, dir string) {
				setReadOnlyDirectoryACL(t, dir, true)
			},
		},
		{
			name: "unprotected DACL with inherited ACEs",
			configure: func(t *testing.T, dir string) {
				setReadOnlyDirectoryACL(t, dir, false)
			},
		},
		{
			name: "empty DACL",
			configure: func(t *testing.T, dir string) {
				emptyACLBytes := []byte{2, 0, 8, 0, 0, 0, 0, 0}
				emptyACL := (*windows.ACL)(unsafe.Pointer(&emptyACLBytes[0]))
				if err := windows.SetNamedSecurityInfo(dir, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION, nil, nil, emptyACL, nil); err != nil {
					t.Fatalf("SetNamedSecurityInfo(empty DACL) error = %v", err)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			requirePersistentACL(t, dir)
			restoreDACL(t, dir)
			tt.configure(t, dir)
			originalACL := captureDACL(t, dir)
			initialErr := &os.PathError{Op: "createtemp", Path: dir, Err: windows.ERROR_ACCESS_DENIED}
			createCalls := 0
			tmp, restore, err := createAtomicTempWith(dir, filepath.Join(dir, "target.txt"), func(dir, pattern string) (*os.File, error) {
				createCalls++
				if createCalls == 1 {
					return nil, initialErr
				}
				return os.CreateTemp(dir, pattern)
			})
			if err != nil {
				t.Fatalf("createAtomicTempWith() error = %v", err)
			}
			tmpPath := tmp.Name()
			t.Cleanup(func() { os.Remove(tmpPath) })
			if err := tmp.Close(); err != nil {
				t.Fatalf("Close() error = %v", err)
			}
			if err := restore(); err != nil {
				t.Fatalf("restore() error = %v", err)
			}
			assertDACLMatches(t, dir, originalACL)
		})
	}
}

func TestCreateAtomicTempLeavesACLUnchangedAfterInstallFailure(t *testing.T) {
	dir := t.TempDir()
	requirePersistentACL(t, dir)
	restoreDACL(t, dir)
	setReadOnlyDirectoryACL(t, dir, true)
	originalACL := captureDACL(t, dir)
	installErr := errors.New("install ACL failed")

	_, _, err := createAtomicTempWithACL(
		dir,
		filepath.Join(dir, "target.txt"),
		func(string, string) (*os.File, error) {
			return nil, &os.PathError{Op: "createtemp", Path: dir, Err: windows.ERROR_ACCESS_DENIED}
		},
		func(string) (func() error, error) {
			return nil, installErr
		},
	)
	if !errors.Is(err, installErr) {
		t.Fatalf("createAtomicTempWithACL() error = %v, want install error %v", err, installErr)
	}
	assertDACLMatches(t, dir, originalACL)
}

func TestCreateAtomicTempRetriesACLRestoreAfterRetryFailure(t *testing.T) {
	dir := t.TempDir()
	requirePersistentACL(t, dir)
	restoreDACL(t, dir)
	setReadOnlyDirectoryACL(t, dir, true)
	originalACL := captureDACL(t, dir)
	retryErr := errors.New("temp retry failed")
	restoreErr := errors.New("restore ACL failed")
	createCalls := 0
	setCalls := 0

	_, _, err := createAtomicTempWithACL(
		dir,
		filepath.Join(dir, "target.txt"),
		func(string, string) (*os.File, error) {
			createCalls++
			if createCalls == 1 {
				return nil, &os.PathError{Op: "createtemp", Path: dir, Err: windows.ERROR_ACCESS_DENIED}
			}
			return nil, retryErr
		},
		func(dir string) (func() error, error) {
			return relaxAtomicDirectoryACLWith(dir, func(path string, securityInfo windows.SECURITY_INFORMATION, dacl *windows.ACL) error {
				setCalls++
				if setCalls == 2 {
					return restoreErr
				}
				return windows.SetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, securityInfo, nil, nil, dacl, nil)
			})
		},
	)
	if !errors.Is(err, retryErr) || !errors.Is(err, restoreErr) {
		t.Fatalf("createAtomicTempWithACL() error = %v, want retry error %v and restore error %v", err, retryErr, restoreErr)
	}
	if setCalls != 3 {
		t.Fatalf("SetNamedSecurityInfo calls = %d, want install plus two restore attempts", setCalls)
	}
	assertDACLMatches(t, dir, originalACL)
}

func TestWriteFileAtomicFailsClosedAfterACLRestoreFailure(t *testing.T) {
	dir := t.TempDir()
	requirePersistentACL(t, dir)
	restoreDACL(t, dir)
	setReadOnlyDirectoryACL(t, dir, true)
	originalACL := captureDACL(t, dir)
	path := filepath.Join(dir, "target.txt")
	restoreErr := errors.New("restore ACL failed")
	setCalls := 0
	operations := defaultAtomicWriteOperations
	operations.createTemp = func(dir, target string) (*os.File, func() error, error) {
		return createAtomicTempWithACL(dir, target, os.CreateTemp, func(dir string) (func() error, error) {
			restore, err := relaxAtomicDirectoryACLWith(dir, func(path string, securityInfo windows.SECURITY_INFORMATION, dacl *windows.ACL) error {
				setCalls++
				if setCalls == 2 {
					return restoreErr
				}
				return windows.SetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, securityInfo, nil, nil, dacl, nil)
			})
			return restore, err
		})
	}

	_, err := writeFileAtomic(path, []byte("new\n"), 0o644, operations)
	if !errors.Is(err, restoreErr) {
		t.Fatalf("writeFileAtomic() error = %v, want restore error %v", err, restoreErr)
	}
	if setCalls != 3 {
		t.Fatalf("SetNamedSecurityInfo calls = %d, want install plus two restore attempts", setCalls)
	}
	assertDACLMatches(t, dir, originalACL)
	got, readErr := os.ReadFile(path)
	if readErr != nil {
		t.Fatalf("ReadFile(target) error = %v", readErr)
	}
	if string(got) != "new\n" {
		t.Fatalf("target content = %q, want completed atomic replacement", got)
	}
}

func TestWriteFileAtomicRestoresWindowsDirectoryACLAfterOperationFailures(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*atomicWriteOperations, error)
	}{
		{
			name: "write",
			mutate: func(operations *atomicWriteOperations, wantErr error) {
				operations.write = func(*os.File, []byte) (int, error) { return 0, wantErr }
			},
		},
		{
			name: "chmod",
			mutate: func(operations *atomicWriteOperations, wantErr error) {
				operations.chmod = func(*os.File, fs.FileMode) error { return wantErr }
			},
		},
		{
			name: "sync",
			mutate: func(operations *atomicWriteOperations, wantErr error) {
				operations.sync = func(*os.File) error { return wantErr }
			},
		},
		{
			name: "close",
			mutate: func(operations *atomicWriteOperations, wantErr error) {
				operations.close = func(file *os.File) error {
					if err := file.Close(); err != nil {
						return err
					}
					return wantErr
				}
			},
		},
		{
			name: "rename",
			mutate: func(operations *atomicWriteOperations, wantErr error) {
				operations.rename = func(string, string) error { return wantErr }
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			requirePersistentACL(t, dir)
			restoreDACL(t, dir)
			path := filepath.Join(dir, "existing.txt")
			originalContent := []byte("original\n")
			if err := os.WriteFile(path, originalContent, 0o644); err != nil {
				t.Fatalf("WriteFile() error = %v", err)
			}
			setReadOnlyDirectoryACL(t, dir, true)
			setCurrentUserACL(t, path, windows.GENERIC_ALL, true)
			originalACL := captureDACL(t, dir)
			wantErr := errors.New(tt.name + " failed")
			operations := defaultAtomicWriteOperations
			operations.createTemp = createAtomicTempWithRelaxedACL
			tt.mutate(&operations, wantErr)

			_, err := writeFileAtomic(path, []byte("replacement\n"), 0o644, operations)
			if !errors.Is(err, wantErr) {
				t.Fatalf("writeFileAtomic() error = %v, want %v", err, wantErr)
			}
			assertDACLMatches(t, dir, originalACL)
			got, readErr := os.ReadFile(path)
			if readErr != nil {
				t.Fatalf("ReadFile() error = %v", readErr)
			}
			if !bytes.Equal(got, originalContent) {
				t.Fatalf("file content = %q, want unchanged %q", got, originalContent)
			}
		})
	}
}

func TestWriteFileAtomicRestoresWindowsDirectoryACLWhenReplacingTarget(t *testing.T) {
	dir := t.TempDir()
	requirePersistentACL(t, dir)
	restoreDACL(t, dir)
	path := filepath.Join(dir, "existing.txt")
	if err := os.WriteFile(path, []byte("original\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	setReadOnlyDirectoryACL(t, dir, true)
	setCurrentUserACL(t, path, windows.GENERIC_ALL, true)
	originalACL := captureDACL(t, dir)

	content := []byte("replacement\n")
	operations := defaultAtomicWriteOperations
	operations.createTemp = createAtomicTempWithRelaxedACL
	if _, err := writeFileAtomic(path, content, 0o644, operations); err != nil {
		t.Fatalf("writeFileAtomic() error = %v, want successful replacement", err)
	}
	assertDACLMatches(t, dir, originalACL)
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if !bytes.Equal(got, content) {
		t.Fatalf("file content = %q, want %q", got, content)
	}
}

func TestCreateAtomicTempPreservesErrorAndPresentNullDACL(t *testing.T) {
	dir := t.TempDir()
	requirePersistentACL(t, dir)
	restoreDACL(t, dir)
	if err := windows.SetNamedSecurityInfo(
		dir,
		windows.SE_FILE_OBJECT,
		windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION,
		nil,
		nil,
		nil,
		nil,
	); err != nil {
		t.Fatalf("SetNamedSecurityInfo(NULL DACL fixture) error = %v", err)
	}
	assertPresentNullDACL(t, dir)
	if _, err := relaxAtomicDirectoryACL(dir); err == nil {
		t.Fatal("relaxAtomicDirectoryACL() error = nil, want present NULL DACL rejected")
	}
	assertPresentNullDACL(t, dir)

	initialErr := &os.PathError{Op: "createtemp", Path: dir, Err: windows.ERROR_ACCESS_DENIED}
	createCalls := 0
	_, _, err := createAtomicTempWith(dir, filepath.Join(dir, "target.txt"), func(string, string) (*os.File, error) {
		createCalls++
		return nil, initialErr
	})
	if err != initialErr {
		t.Fatalf("createAtomicTempWith() error = %v, want original error %v", err, initialErr)
	}
	if createCalls != 1 {
		t.Fatalf("CreateTemp calls = %d, want 1", createCalls)
	}
	assertPresentNullDACL(t, dir)
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
	operations := defaultAtomicWriteOperations
	operations.createTemp = createAtomicTempWithRelaxedACL
	if _, err := writeFileAtomic(path, content, 0o644, operations); err != nil {
		t.Fatalf("writeFileAtomic() error = %v, want unrelated deny ignored", err)
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

type daclSnapshot struct {
	present bool
	null    bool
	control windows.SECURITY_DESCRIPTOR_CONTROL
	acl     []byte
}

type aclHeader struct {
	revision uint8
	reserved uint8
	size     uint16
	aceCount uint16
	padding  uint16
}

func captureDACL(t *testing.T, path string) daclSnapshot {
	t.Helper()
	descriptor, err := windows.GetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		t.Fatalf("GetNamedSecurityInfo(%q) error = %v", path, err)
	}
	dacl, _, err := descriptor.DACL()
	if err != nil {
		t.Fatalf("SECURITY_DESCRIPTOR.DACL(%q) error = %v", path, err)
	}
	control, _, err := descriptor.Control()
	if err != nil {
		t.Fatalf("SECURITY_DESCRIPTOR.Control(%q) error = %v", path, err)
	}
	snapshot := daclSnapshot{
		present: control&windows.SE_DACL_PRESENT != 0,
		null:    dacl == nil,
		control: control,
	}
	if dacl != nil {
		header := (*aclHeader)(unsafe.Pointer(dacl))
		snapshot.acl = bytes.Clone(unsafe.Slice((*byte)(unsafe.Pointer(dacl)), int(header.size)))
	}
	return snapshot
}

func assertDACLMatches(t *testing.T, path string, want daclSnapshot) {
	t.Helper()
	got := captureDACL(t, path)
	if got.present != want.present || got.null != want.null || got.control != want.control || !bytes.Equal(got.acl, want.acl) {
		t.Fatalf("DACL changed for %q:\n got present=%t null=%t control=%#x acl=%x\nwant present=%t null=%t control=%#x acl=%x", path, got.present, got.null, got.control, got.acl, want.present, want.null, want.control, want.acl)
	}
}

func setReadOnlyDirectoryACL(t *testing.T, path string, protected bool) {
	t.Helper()
	setCurrentUserACL(t, path, windows.GENERIC_READ|windows.GENERIC_EXECUTE, protected)
}

func setCurrentUserACL(t *testing.T, path string, permissions windows.ACCESS_MASK, protected bool) {
	t.Helper()
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		t.Fatalf("GetTokenUser() error = %v", err)
	}
	dacl, err := windows.ACLFromEntries([]windows.EXPLICIT_ACCESS{{
		AccessPermissions: permissions,
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
	securityInfo := windows.SECURITY_INFORMATION(windows.DACL_SECURITY_INFORMATION | windows.UNPROTECTED_DACL_SECURITY_INFORMATION)
	if protected {
		securityInfo = windows.SECURITY_INFORMATION(windows.DACL_SECURITY_INFORMATION | windows.PROTECTED_DACL_SECURITY_INFORMATION)
	}
	if err := windows.SetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, securityInfo, nil, nil, dacl, nil); err != nil {
		t.Fatalf("SetNamedSecurityInfo(read-only fixture) error = %v", err)
	}
}

func assertPresentNullDACL(t *testing.T, path string) {
	t.Helper()
	descriptor, err := windows.GetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		t.Fatalf("GetNamedSecurityInfo() error = %v", err)
	}
	dacl, _, err := descriptor.DACL()
	if err != nil {
		t.Fatalf("SECURITY_DESCRIPTOR.DACL() error = %v", err)
	}
	if dacl != nil {
		t.Fatalf("DACL = %p, want present NULL DACL", dacl)
	}
}

func aclGrantsRightsToSID(dacl *windows.ACL, sid *windows.SID, rights windows.ACCESS_MASK) (bool, error) {
	for i := uint32(0); i < uint32(dacl.AceCount); i++ {
		var ace *windows.ACCESS_ALLOWED_ACE
		if err := windows.GetAce(dacl, i, &ace); err != nil {
			return false, err
		}
		if ace.Header.AceType != windows.ACCESS_ALLOWED_ACE_TYPE || ace.Header.AceFlags&windows.INHERIT_ONLY_ACE != 0 {
			continue
		}
		aceSID := (*windows.SID)(unsafe.Pointer(&ace.SidStart))
		if aceSID.Equals(sid) && ace.Mask&rights != 0 {
			return true, nil
		}
	}
	return false, nil
}
