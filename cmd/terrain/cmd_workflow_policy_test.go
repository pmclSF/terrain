package main

import "testing"

// TestCountPolicyOverrides covers the parser used by the
// `terrain doctor` policy-overrides check. Commented-out starter
// policy reports zero overrides; explicit actions tally per kind.
func TestCountPolicyOverrides(t *testing.T) {
	cases := []struct {
		name                       string
		body                       string
		wantOff, wantDem, wantProm int
	}{
		{
			name: "empty body",
			body: ``,
		},
		{
			name: "all comments (starter policy)",
			body: `# rules:
#   - rule: "weakAssertion"
#     action: off
`,
		},
		{
			name: "two off",
			body: `rules:
  - rule: "weakAssertion"
    action: off
  - rule: "untestedExport"
    action: off
`,
			wantOff: 2,
		},
		{
			name: "mixed actions",
			body: `rules:
  - rule: "a"
    action: off
  - rule: "b"
    action: demote
  - rule: "c"
    action: promote
`,
			wantOff: 1, wantDem: 1, wantProm: 1,
		},
		{
			name: "synonyms tally to canonical bucket",
			body: `rules:
  - rule: "a"
    action: disable
  - rule: "b"
    action: warn
  - rule: "c"
    action: block
`,
			wantOff: 1, wantDem: 1, wantProm: 1,
		},
		{
			name: "quoted values still parse",
			body: `rules:
  - rule: "a"
    action: "off"
`,
			wantOff: 1,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			off, dem, prom := countPolicyOverrides(tc.body)
			if off != tc.wantOff || dem != tc.wantDem || prom != tc.wantProm {
				t.Errorf("got (off=%d dem=%d prom=%d), want (off=%d dem=%d prom=%d)",
					off, dem, prom, tc.wantOff, tc.wantDem, tc.wantProm)
			}
		})
	}
}
