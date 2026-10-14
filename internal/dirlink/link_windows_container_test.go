//go:build windows && container_fs

package dirlink

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/sys/windows"
)

func TestOfflineUNCTargetUsesDirectorySymlink(t *testing.T) {
	link := filepath.Join(t.TempDir(), "offline-network-config")
	target := `\\127.0.0.1\fest-offline-share\config`
	if err := Create(link, target); err != nil {
		if strings.Contains(err.Error(), "Windows blocked unprivileged symlink creation") {
			if _, statErr := os.Lstat(link); !os.IsNotExist(statErr) {
				t.Fatalf("denied symlink left a link behind: %v", statErr)
			}
			t.Logf("Windows denied directory symlink creation as expected: %v", err)
			return
		}
		t.Fatalf("Create offline UNC link: %v", err)
	}
	defer func() { _ = Remove(link) }()

	kind, err := Kind(link)
	if err != nil || kind != KindSymlink {
		t.Fatalf("link kind = %v, %v; want directory symlink", kind, err)
	}
	ptr, err := windows.UTF16PtrFromString(link)
	if err != nil {
		t.Fatal(err)
	}
	attrs, err := windows.GetFileAttributes(ptr)
	if err != nil {
		t.Fatal(err)
	}
	if attrs&windows.FILE_ATTRIBUTE_DIRECTORY == 0 {
		t.Fatalf("offline UNC link lacks directory attribute: %#x", attrs)
	}
	if got, err := ReadTarget(link); err != nil || !strings.EqualFold(got, target) {
		t.Fatalf("ReadTarget = %q, %v; want %q", got, err, target)
	}
	if err := Remove(link); err != nil {
		t.Fatalf("Remove offline UNC link: %v", err)
	}
	if _, err := os.Lstat(link); !os.IsNotExist(err) {
		t.Fatalf("offline UNC link still exists: %v", err)
	}
}
