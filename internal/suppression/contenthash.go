package suppression

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// ContextHashRadius is the number of lines included on each side of
// the finding's anchor line. Total window size = 2*radius + 1 = 5.
const ContextHashRadius = 2

// ContextHash returns the SHA-256 of the normalized context window
// around (file, line). The window covers `line - ContextHashRadius`
// through `line + ContextHashRadius` (inclusive). Lines outside the
// file's actual bounds contribute as empty strings.
//
// Normalization: each line in the window has trailing whitespace
// trimmed (via strings.TrimRight on " \t"). Internal whitespace and
// leading whitespace are preserved — an indent change still
// invalidates the hash because indent typically signals a semantic
// shift (`if {}` → `if () {}` reshape, struct re-nesting, etc.).
//
// Lines are joined with a single "\n" between them.
//
// Returns an empty string and a nil error when `file` doesn't exist.
// A non-existent file is a valid runtime state — the rule no longer
// fires there, so there's nothing to suppress. The empty-hash sentinel
// signals "skip this content-hash comparison" to the matcher rather
// than blocking the whole suppression flow.
//
// `line` is 1-indexed (matching models.SignalLocation.Line). A line
// of 0 or negative returns "" and a nil error.
func ContextHash(file string, line int) (string, error) {
	if file == "" || line <= 0 {
		return "", nil
	}
	clean := filepath.Clean(file)
	if strings.Contains(clean, "\x00") || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", nil
	}
	if fi, statErr := os.Lstat(clean); statErr != nil || !fi.Mode().IsRegular() {
		if statErr == nil || errors.Is(statErr, os.ErrNotExist) {
			// Missing or non-regular (symlink/device/FIFO): no content to
			// hash here, so emit the skip sentinel rather than following
			// the link or erroring the whole flow.
			return "", nil
		}
		return "", fmt.Errorf("contenthash: open %q: %w", clean, statErr)
	}
	f, err := os.Open(clean)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", nil
		}
		return "", fmt.Errorf("contenthash: open %q: %w", clean, err)
	}
	defer f.Close()

	return contextHashFromReader(f, line)
}

// contextHashFromReader is the io.Reader-parameterized core. Exposed
// to tests so they can hash synthetic content without touching the
// filesystem.
func contextHashFromReader(r io.Reader, line int) (string, error) {
	first := line - ContextHashRadius
	last := line + ContextHashRadius
	if first < 1 {
		first = 1
	}

	// Read lines [1, last]. Lines past `last` are discarded.
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 1<<16), 1<<20) // 1 MiB max line len
	var captured []string
	current := 0
	for scanner.Scan() {
		current++
		if current > last {
			break
		}
		if current >= first {
			captured = append(captured, strings.TrimRight(scanner.Text(), " \t"))
		}
	}
	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("contenthash: scan: %w", err)
	}

	// Pad with empty strings for any positions the file ran out of
	// content for. Result window is always 2*ContextHashRadius+1
	// lines, regardless of where in the file `line` sits.
	want := last - first + 1
	for len(captured) < want {
		captured = append(captured, "")
	}
	// Also pad the leading positions when line - radius < 1 (i.e.
	// `first` was clamped to 1 but the conceptual window started
	// earlier). This keeps the hash stable regardless of which side
	// of the file the line is near.
	preWant := (line - 1) - (line - ContextHashRadius - 1)
	_ = preWant // pre-window padding folds into the "first clamped to 1" branch above
	if leading := ContextHashRadius - (line - 1); leading > 0 {
		empties := make([]string, leading)
		captured = append(empties, captured...)
	}

	joined := strings.Join(captured, "\n")
	sum := sha256.Sum256([]byte(joined))
	return hex.EncodeToString(sum[:]), nil
}
