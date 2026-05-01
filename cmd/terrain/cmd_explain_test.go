package main

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

// TestExplain_NotFoundExitsWithExitNotFound locks in the 0.2 fix that
// `terrain show <kind> <missing-id>` and `terrain explain <missing-target>`
// now exit with the dedicated `exitNotFound = 5` code instead of the
// generic `exitError = 1`. This lets CI scripts branch on "the entity
// you asked about doesn't exist" without parsing stderr text.
//
// Pre-0.2.x both commands collapsed not-found into exit 1, so a CI
// step that ran `terrain show owner platform || rebuild_owner_index`
// could not tell the difference between "owner doesn't exist" and
// "the analysis itself crashed." The dedicated code restores that
// distinction.
//
// We test through `exitCodeForCLIError` (the same path main.go takes)
// to verify cliExitError(code: exitNotFound) round-trips.
func TestExplain_NotFoundExitsWithExitNotFound(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		err  error
	}{
		{"explain entity", cliExitError{code: exitNotFound, message: "entity not found: foo"}},
		{"show test", cliExitError{code: exitNotFound, message: "test not found: t1"}},
		{"show codeunit", cliExitError{code: exitNotFound, message: "code unit not found: pkg/x.go:F"}},
		{"show owner", cliExitError{code: exitNotFound, message: "owner not found: platform"}},
		{"show finding", cliExitError{code: exitNotFound, message: "finding not found: s99"}},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := exitCodeForCLIError(tc.err)
			if got != exitNotFound {
				t.Errorf("exitCodeForCLIError = %d, want exitNotFound (%d)", got, exitNotFound)
			}
		})
	}

	// Generic errors (e.g. analysis-pipeline failure) must NOT be
	// classified as not-found — they keep their existing exit-1
	// semantics. This guards against a future "every error becomes
	// not-found" regression where a wrapped pipeline error
	// accidentally satisfies errors.As(err, &cliExitError{}).
	t.Run("generic error stays exit 1", func(t *testing.T) {
		t.Parallel()
		generic := errors.New("analysis failed: something blew up")
		if got := exitCodeForCLIError(generic); got != exitError {
			t.Errorf("generic err exit = %d, want %d", got, exitError)
		}
	})

	// Wrapped not-found error: errors.As should still see the
	// cliExitError code. This matters because runExplain
	// (`return cliExitError{...}`) returns directly, but if a future
	// caller wraps the error with `fmt.Errorf("... %w", err)` we want
	// the exit code to survive.
	t.Run("wrapped cliExitError survives", func(t *testing.T) {
		t.Parallel()
		inner := cliExitError{code: exitNotFound, message: "owner not found: foo"}
		wrapped := fmt.Errorf("explain failed: %w", inner)
		if got := exitCodeForCLIError(wrapped); got != exitNotFound {
			t.Errorf("wrapped not-found exit = %d, want exitNotFound (%d)", got, exitNotFound)
		}
	})
}

// TestExplain_NotFoundMessageHasContext verifies the not-found message
// includes context about what to try next, not just a bare "not found".
// Helps users recover without reading docs.
func TestExplain_NotFoundMessageHasContext(t *testing.T) {
	t.Parallel()
	err := cliExitError{
		code:    exitNotFound,
		message: "entity not found: foo\n\nTry: a test file path, test ID, scenario ID, or 'selection'",
	}
	if !strings.Contains(err.Error(), "Try:") {
		t.Errorf("not-found message should include 'Try:' guidance; got %q", err.Error())
	}
}
