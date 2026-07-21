// Package saferead reads repository files defensively during walks over
// untrusted repositories. Terrain analyzes whatever repo it is pointed at, so
// a scanned tree may contain hostile or pathological entries: a source file
// symlinked to /dev/zero, a manifest that is actually a multi-gigabyte blob, a
// named pipe where a config is expected. A naive os.ReadFile follows symlinks
// and grows its buffer to the size of whatever it finds, so a single crafted
// entry can exhaust memory before any analysis runs.
//
// File centralizes the guard: reject anything that is not a regular file
// within a byte cap, using Lstat so a symlink is judged on its own type
// without being followed to its target.
package saferead

import (
	"fmt"
	"os"
)

// SourceCap bounds hand-written source and config files (256 KB). Real prompt
// code, schemas, and manifests are a few KB; anything larger is generated,
// vendored, or hostile. The cap also bounds parse cost — tree-sitter parse
// time is superlinear in file size, so an uncapped multi-MB file would stall a
// scan for minutes.
const SourceCap = 256 * 1024

// DataCap bounds machine-generated data files that are legitimately large
// (dbt manifests, OpenAPI specs, SQL dumps) while still rejecting a
// pathological multi-gigabyte file.
const DataCap = 64 * 1024 * 1024

// File reads path only when it is a regular file no larger than cap bytes.
// It returns ok == false — and the caller skips the entry — for symlinks
// (including one pointing at /dev/zero or a huge file), devices, FIFOs,
// directories, oversize files, or any stat/read error. Lstat is used so the
// symlink itself is inspected, never followed.
func File(path string, cap int64) ([]byte, bool) {
	fi, err := os.Lstat(path)
	if err != nil || !fi.Mode().IsRegular() || fi.Size() > cap {
		return nil, false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, false
	}
	return data, true
}

// ReadFile is a drop-in replacement for os.ReadFile for reads of paths
// discovered while walking an untrusted repository. It has os.ReadFile's exact
// ([]byte, error) signature — so a caller's existing `if err != nil { skip }`
// handling keeps working — but returns an error, without touching the bytes,
// when the path is not a regular file (a symlink to /dev/zero, a device, a
// FIFO) or exceeds SourceCap. Use ReadFileCap for legitimately large data
// files.
func ReadFile(path string) ([]byte, error) {
	return ReadFileCap(path, SourceCap)
}

// ReadFileCap is ReadFile with a caller-chosen byte cap. Pass DataCap for
// machine-generated artifacts (dbt manifests, OpenAPI specs, SQL dumps) that
// are legitimately larger than hand-written source.
func ReadFileCap(path string, cap int64) ([]byte, error) {
	fi, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !fi.Mode().IsRegular() {
		return nil, fmt.Errorf("saferead: %s is not a regular file", path)
	}
	if fi.Size() > cap {
		return nil, fmt.Errorf("saferead: %s exceeds the %d-byte read cap", path, cap)
	}
	return os.ReadFile(path)
}
