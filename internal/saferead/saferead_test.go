package saferead

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestFile_RegularWithinCap(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	p := filepath.Join(dir, "ok.py")
	if err := os.WriteFile(p, []byte("import openai\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	data, ok := File(p, SourceCap)
	if !ok || string(data) != "import openai\n" {
		t.Fatalf("regular file should read: ok=%v data=%q", ok, data)
	}
}

func TestFile_OversizeRejected(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	p := filepath.Join(dir, "big.py")
	if err := os.WriteFile(p, make([]byte, SourceCap+1), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, ok := File(p, SourceCap); ok {
		t.Fatal("file over cap should be rejected")
	}
	// The same bytes are fine under a larger cap.
	if _, ok := File(p, DataCap); !ok {
		t.Fatal("file under DataCap should read")
	}
}

func TestFile_SymlinkRejectedNotFollowed(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("symlink semantics differ on windows")
	}
	dir := t.TempDir()
	target := filepath.Join(dir, "target.py")
	if err := os.WriteFile(target, []byte("import openai\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "link.py")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	// A symlink to a small regular file is still rejected — we never follow it,
	// so we never read an attacker-controlled target (which could be /dev/zero).
	if _, ok := File(link, SourceCap); ok {
		t.Fatal("symlink must be rejected without following")
	}
}

func TestFile_DeviceRejected(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("no /dev/zero on windows")
	}
	// The exact vector from the robustness review: reading /dev/zero directly
	// would grow the buffer without bound. Lstat sees a device, not a regular
	// file, so File refuses it.
	if _, ok := File("/dev/zero", SourceCap); ok {
		t.Fatal("/dev/zero (character device) must be rejected")
	}
}

func TestFile_MissingReturnsFalse(t *testing.T) {
	t.Parallel()
	if _, ok := File(filepath.Join(t.TempDir(), "nope.py"), SourceCap); ok {
		t.Fatal("missing file should return ok=false")
	}
}

func TestReadFile_DropInMatchesOSForRegularFiles(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	p := filepath.Join(dir, "ok.py")
	if err := os.WriteFile(p, []byte("import openai\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	data, err := ReadFile(p)
	if err != nil || string(data) != "import openai\n" {
		t.Fatalf("ReadFile should behave like os.ReadFile on a regular file: err=%v data=%q", err, data)
	}
}

func TestReadFile_DeviceReturnsError(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("no /dev/zero on windows")
	}
	// The drop-in must error (not hang) on /dev/zero so a caller's existing
	// `if err != nil { skip }` path handles it.
	if _, err := ReadFile("/dev/zero"); err == nil {
		t.Fatal("ReadFile(/dev/zero) must return an error, not read unbounded")
	}
}

func TestReadFileCap_HonorsCap(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	p := filepath.Join(dir, "data.json")
	if err := os.WriteFile(p, make([]byte, SourceCap+1), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadFile(p); err == nil {
		t.Fatal("ReadFile should reject a file over SourceCap")
	}
	if _, err := ReadFileCap(p, DataCap); err != nil {
		t.Fatalf("ReadFileCap(DataCap) should accept it: %v", err)
	}
}
