package slash

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
)

// VerifySignature validates a GitHub webhook payload's HMAC-SHA256
// signature. GitHub computes the signature as
// `sha256=<hex(hmac_sha256(secret, body))>` and sends it in the
// `X-Hub-Signature-256` header. This function returns nil on a
// valid signature, a non-nil error otherwise.
//
// Uses constant-time comparison (hmac.Equal) so timing-side-channel
// attacks can't leak the secret one byte at a time.
//
// Empty secret returns an error explicitly — silently accepting an
// unsigned webhook is the wrong default.
func VerifySignature(headerValue string, body []byte, secret string) error {
	if secret == "" {
		return fmt.Errorf("verify signature: webhook secret not configured")
	}
	const prefix = "sha256="
	if !strings.HasPrefix(headerValue, prefix) {
		return fmt.Errorf("verify signature: header missing 'sha256=' prefix")
	}
	sig := headerValue[len(prefix):]
	want, err := hex.DecodeString(sig)
	if err != nil {
		return fmt.Errorf("verify signature: header is not hex: %w", err)
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	got := mac.Sum(nil)
	if !hmac.Equal(want, got) {
		return fmt.Errorf("verify signature: signature mismatch")
	}
	return nil
}

// ComputeSignatureHeader is the symmetric helper for tests: given a
// body and secret, return the `sha256=...` header value GitHub
// would send. Production code never calls this.
func ComputeSignatureHeader(body []byte, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}
