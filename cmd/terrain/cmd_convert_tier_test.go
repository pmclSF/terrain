package main

import (
	"testing"

	conv "github.com/pmclSF/terrain/internal/convert"
)

// TestTierLabelForState locks the GoNativeState → Tier-label mapping
// that surfaces in `terrain migrate list` output. The labels are the
// canonical user-facing vocabulary; renaming a label here is a
// public-facing change and requires updating the alignment-first
// migration docs too.
func TestTierLabelForState(t *testing.T) {
	t.Parallel()
	tests := []struct {
		state conv.GoNativeState
		want  string
	}{
		{conv.GoNativeStateImplemented, "Stable"},
		{conv.GoNativeStateExperimental, "Experimental"},
		{conv.GoNativeStatePrioritized, "Preview"},
		{conv.GoNativeStateCataloged, "Cataloged"},
		{conv.GoNativeState("unknown"), "Cataloged"},
	}
	for _, tt := range tests {
		t.Run(string(tt.state), func(t *testing.T) {
			t.Parallel()
			if got := tierLabelForState(tt.state); got != tt.want {
				t.Errorf("tierLabelForState(%q) = %q, want %q", tt.state, got, tt.want)
			}
		})
	}
}
