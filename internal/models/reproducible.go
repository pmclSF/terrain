package models

import (
	"os"
	"strconv"
	"strings"
	"time"
)

// DeterministicNowUTC returns time.Now().UTC() unless SOURCE_DATE_EPOCH is set
// (the Reproducible Builds standard, https://reproducible-builds.org), in which
// case it returns that epoch. Honouring it lets every wall-clock reference in
// Terrain's emitted artifacts be byte-comparable across runs — the
// deterministic-output guarantee. Any surface that stamps a timestamp into
// output must use this rather than time.Now, so two runs on the same commit
// produce identical bytes under SOURCE_DATE_EPOCH.
func DeterministicNowUTC() time.Time {
	if v := os.Getenv("SOURCE_DATE_EPOCH"); v != "" {
		if secs, err := strconv.ParseInt(strings.TrimSpace(v), 10, 64); err == nil {
			return time.Unix(secs, 0).UTC()
		}
	}
	return time.Now().UTC()
}
