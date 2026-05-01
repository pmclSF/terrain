package expiry

import "time"

// ShouldExpire reports whether the cache entry placed at ts has aged
// beyond ttl. Lives in a subpackage with no tests so neither the
// import-graph linkage nor the directory-proximity heuristic covers
// it.
func ShouldExpire(ts time.Time, ttl time.Duration) bool {
	return time.Since(ts) > ttl
}
