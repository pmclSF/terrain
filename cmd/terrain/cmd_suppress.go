package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pmclSF/terrain/internal/identity"
	"github.com/pmclSF/terrain/internal/suppression"
)

// runSuppress writes a new entry into `.terrain/suppressions.yaml`
// for the given finding ID. The flow is:
//
//  1. Validate the finding ID parses (Track 4.4 format).
//  2. Load the existing suppressions file (or create an empty one).
//  3. Refuse to add a duplicate entry — if one already exists, print
//     a helpful message pointing at the existing reason and exit.
//  4. Append the new entry.
//  5. Write back, preserving comments and ordering of existing
//     entries via simple text-append (we deliberately don't round-trip
//     through YAML because goccy/go-yaml's comment preservation is
//     uneven; appending text is the safer 0.2.0 shape).
//
// Result: a suppression entry with reason + optional expires + owner,
// ready for the next `terrain analyze` run to honor.
func runSuppress(findingID, reason, expires, owner, root string) error {
	if findingID == "" {
		return fmt.Errorf("missing finding ID — usage: terrain suppress <finding-id> --reason \"why\"")
	}
	if _, _, _, _, ok := identity.ParseFindingID(findingID); !ok {
		return fmt.Errorf("invalid finding ID format %q — expected detector@path:anchor#hash", findingID)
	}
	if strings.TrimSpace(reason) == "" {
		return fmt.Errorf("--reason is required (every suppression must justify itself)")
	}

	// Light sanity-check on `expires` — if the user supplied something,
	// it should at least look like an ISO date so a downstream parser
	// doesn't trip silently. We don't enforce real-date validity here;
	// the loader emits a non-fatal warning if it can't parse.
	if expires != "" {
		if !looksLikeISODate(expires) {
			return fmt.Errorf("--expires %q does not look like YYYY-MM-DD", expires)
		}
	}

	suppPath := filepath.Join(root, suppression.DefaultPath)

	// Check for existing entry — refuse to add duplicates so users
	// don't accidentally accumulate stale waivers.
	existing, err := suppression.Load(suppPath)
	if err != nil {
		return fmt.Errorf("could not load existing %s: %w", suppression.DefaultPath, err)
	}
	if existing != nil {
		for _, e := range existing.Entries {
			if e.FindingID == findingID {
				return cliExitError{
					code: exitUsageError,
					message: fmt.Sprintf(
						"finding %s is already suppressed.\n"+
							"Existing reason: %s\n"+
							"Existing owner:  %s\n"+
							"Existing expires: %s\n\n"+
							"Edit %s directly to update the entry, or remove it first to re-add.",
						findingID, e.Reason, e.Owner, e.Expires, suppPath,
					),
				}
			}
		}
	}

	// Build the entry as YAML text. Append to the existing file, or
	// create a new file with the schema header if the file doesn't
	// exist. We deliberately write text rather than re-marshaling —
	// preserves any comments / ordering the user added by hand.
	if err := os.MkdirAll(filepath.Dir(suppPath), 0o755); err != nil {
		return fmt.Errorf("could not create %s parent dir: %w", suppression.DefaultPath, err)
	}

	header := ""
	body, readErr := os.ReadFile(suppPath)
	if readErr != nil {
		if !os.IsNotExist(readErr) {
			return fmt.Errorf("read %s: %w", suppPath, readErr)
		}
		// New file — emit the schema header.
		header = "# Terrain suppressions — generated and edited by `terrain suppress`.\n" +
			"# Schema: https://github.com/pmclSF/terrain/blob/main/internal/suppression/suppression.go\n\n" +
			"schema_version: \"1\"\n" +
			"suppressions:\n"
	}

	entry := buildSuppressionYAML(findingID, reason, expires, owner)

	out := header
	if len(body) > 0 {
		out += string(body)
		// Ensure separation before our append.
		if !strings.HasSuffix(out, "\n") {
			out += "\n"
		}
	}
	// If the existing file doesn't yet have a `suppressions:` key, the
	// loader (when it sees only schema_version + an entry) would still
	// parse — but for hygiene, rewrite the whole file with normal shape
	// when we're appending.
	if header == "" && !strings.Contains(string(body), "\nsuppressions:") && !strings.HasPrefix(string(body), "suppressions:") {
		out += "suppressions:\n"
	}
	out += entry

	if err := os.WriteFile(suppPath, []byte(out), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", suppPath, err)
	}

	fmt.Printf("Suppressed %s\n", findingID)
	fmt.Printf("  reason:  %s\n", reason)
	if expires != "" {
		fmt.Printf("  expires: %s\n", expires)
	}
	if owner != "" {
		fmt.Printf("  owner:   %s\n", owner)
	}
	fmt.Printf("\nWritten to: %s\n", suppPath)
	if expires == "" {
		fmt.Println("\nTip: add --expires=YYYY-MM-DD so the suppression doesn't outlive its reason.")
	}
	return nil
}

func buildSuppressionYAML(findingID, reason, expires, owner string) string {
	var b strings.Builder
	b.WriteString("  - finding_id: ")
	b.WriteString(findingID)
	b.WriteString("\n")
	b.WriteString("    reason: ")
	b.WriteString(yamlInlineString(reason))
	b.WriteString("\n")
	if expires != "" {
		b.WriteString("    expires: ")
		b.WriteString(expires)
		b.WriteString("\n")
	}
	if owner != "" {
		b.WriteString("    owner: ")
		b.WriteString(yamlInlineString(owner))
		b.WriteString("\n")
	}
	return b.String()
}

// yamlInlineString quotes a string for safe inline use in YAML.
// We always double-quote so reasons containing special characters
// (`:`, `#`, leading dashes, etc.) round-trip cleanly.
func yamlInlineString(s string) string {
	// Escape backslash + double-quote.
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	return `"` + s + `"`
}

func looksLikeISODate(s string) bool {
	// Cheap shape check: 10 chars in YYYY-MM-DD layout.
	if len(s) != 10 {
		return false
	}
	if s[4] != '-' || s[7] != '-' {
		return false
	}
	for i, r := range s {
		if i == 4 || i == 7 {
			continue
		}
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
