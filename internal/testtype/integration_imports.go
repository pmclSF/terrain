package testtype

import "strings"

// integrationImportPattern matches a literal substring against test-file
// content and, on hit, attributes the file to TypeIntegration with a
// human-readable evidence string. Patterns are intentionally narrow —
// each one targets a library whose presence in a test file is a strong
// signal that the file exercises a real HTTP / RPC / database surface
// rather than a unit under test in isolation.
//
// Why content-based: path/suite/name inference (infer.go) handles
// "test/integration/db.test.js"-style organization but misses the
// common pattern of integration tests living alongside unit tests in a
// flat directory and identifying themselves only through imports
// (a Go file in the same package that imports `net/http/httptest`,
// or a JS file that requires `supertest`). Content-based detection
// closes that gap without forcing teams to reorganize.
type integrationImportPattern struct {
	// Substring is matched literally against the file content.
	// Patterns include surrounding quote / parenthesis characters so
	// substrings that overlap with prose (e.g. the word "request" in
	// a comment) don't match — only real imports do.
	Substring string

	// Library is the canonical library name surfaced as evidence.
	Library string

	// Confidence assigned when the pattern matches. Most libraries
	// score 0.85; libraries that are *almost certainly* integration-
	// only (httptest, MockMvc, supertest) score 0.9.
	Confidence float64
}

// integrationImportPatterns is the explicit allowlist of integration-
// library import signatures. Adding a pattern here is the way to
// extend integration-test classification rigor (Track 3.3).
//
// Conservative by design: false-positive integration claims are
// worse than false negatives because they distort the test-system
// inventory. New patterns should target a library that is either
// purpose-built for integration testing (supertest, httptest) or
// whose presence in a test file overwhelmingly indicates the test
// crosses a real HTTP / database boundary.
var integrationImportPatterns = []integrationImportPattern{
	// JavaScript / TypeScript — HTTP testing libraries.
	{Substring: "require('supertest')", Library: "supertest", Confidence: 0.9},
	{Substring: `require("supertest")`, Library: "supertest", Confidence: 0.9},
	{Substring: "from 'supertest'", Library: "supertest", Confidence: 0.9},
	{Substring: `from "supertest"`, Library: "supertest", Confidence: 0.9},
	{Substring: "require('nock')", Library: "nock", Confidence: 0.85},
	{Substring: `require("nock")`, Library: "nock", Confidence: 0.85},
	{Substring: "from 'nock'", Library: "nock", Confidence: 0.85},
	{Substring: `from "nock"`, Library: "nock", Confidence: 0.85},
	{Substring: "from 'msw'", Library: "msw", Confidence: 0.85},
	{Substring: `from "msw"`, Library: "msw", Confidence: 0.85},
	{Substring: "from 'msw/node'", Library: "msw", Confidence: 0.85},
	{Substring: `from "msw/node"`, Library: "msw", Confidence: 0.85},
	{Substring: "require('pactum')", Library: "pactum", Confidence: 0.9},
	{Substring: `require("pactum")`, Library: "pactum", Confidence: 0.9},
	{Substring: "from 'pactum'", Library: "pactum", Confidence: 0.9},
	{Substring: `from "pactum"`, Library: "pactum", Confidence: 0.9},

	// Go — net/http/httptest is the canonical HTTP-integration import.
	// Match common import shapes: bare import (`"net/http/httptest"`),
	// aliased (`httptest "net/http/httptest"` or `t "net/http/httptest"`),
	// and grouped imports where the line begins with the path string.
	{Substring: `"net/http/httptest"`, Library: "httptest", Confidence: 0.9},

	// Python — requests + pytest is the dominant integration shape.
	// We require both because `import requests` alone is not enough
	// (could be inside a unit test as a mocked client). The classic
	// integration pattern is a pytest test file that imports requests
	// AND defines a function whose body actually calls requests.<verb>.
	// We detect the call shape, not just the import, to keep
	// confidence high.
	{Substring: "requests.get(", Library: "python-requests", Confidence: 0.85},
	{Substring: "requests.post(", Library: "python-requests", Confidence: 0.85},
	{Substring: "requests.put(", Library: "python-requests", Confidence: 0.85},
	{Substring: "requests.delete(", Library: "python-requests", Confidence: 0.85},
	{Substring: "import httpx", Library: "httpx", Confidence: 0.85},
	{Substring: "from httpx", Library: "httpx", Confidence: 0.85},
	{Substring: "import responses", Library: "python-responses", Confidence: 0.85},
	{Substring: "from responses", Library: "python-responses", Confidence: 0.85},
	{Substring: "from pact", Library: "pact-python", Confidence: 0.9},
	{Substring: "import pact", Library: "pact-python", Confidence: 0.9},

	// Java — MockMvc + RestAssured.
	{Substring: "org.springframework.test.web.servlet.MockMvc", Library: "MockMvc", Confidence: 0.9},
	{Substring: "import io.restassured", Library: "RestAssured", Confidence: 0.9},
	{Substring: "import static io.restassured", Library: "RestAssured", Confidence: 0.9},

	// Ruby — rack/test + webmock.
	{Substring: "require 'rack/test'", Library: "rack-test", Confidence: 0.9},
	{Substring: `require "rack/test"`, Library: "rack-test", Confidence: 0.9},
	{Substring: "require 'webmock", Library: "webmock", Confidence: 0.85},
	{Substring: `require "webmock`, Library: "webmock", Confidence: 0.85},

	// Tooling — dredd is API-spec integration.
	{Substring: "dredd-hooks", Library: "dredd", Confidence: 0.9},

	// Database integration — testcontainers is the canonical "real
	// container, not a mock" import across JVM/Go/Node ecosystems.
	{Substring: "testcontainers", Library: "testcontainers", Confidence: 0.85},
}

// InferFromContent scans test-file content for integration-library
// import signatures. Returns TypeUnknown when nothing matches; on
// match returns TypeIntegration with the highest-confidence pattern's
// confidence and per-library evidence.
//
// Returns at most one InferResult — the strongest signal — so callers
// don't need to dedupe libraries themselves. Multiple matched libraries
// are surfaced in the evidence list.
func InferFromContent(content string) InferResult {
	if content == "" {
		return InferResult{Type: TypeUnknown}
	}

	var matched []integrationImportPattern
	for _, p := range integrationImportPatterns {
		if strings.Contains(content, p.Substring) {
			matched = append(matched, p)
		}
	}

	if len(matched) == 0 {
		return InferResult{Type: TypeUnknown}
	}

	best := matched[0]
	for _, m := range matched[1:] {
		if m.Confidence > best.Confidence {
			best = m
		}
	}

	evidence := make([]string, 0, len(matched))
	seen := map[string]bool{}
	for _, m := range matched {
		if seen[m.Library] {
			continue
		}
		seen[m.Library] = true
		evidence = append(evidence, "integration library detected: "+m.Library)
	}

	return InferResult{
		Type:       TypeIntegration,
		Confidence: best.Confidence,
		Evidence:   evidence,
	}
}

// MergeContentInference combines a path/suite/framework-based InferResult
// (from InferForTestCase) with a content-based InferResult (from
// InferFromContent). When both agree on TypeIntegration the result keeps
// the higher confidence and accumulates evidence; when they disagree the
// content-based signal takes precedence because explicit imports are
// harder to fake than directory naming.
//
// Used by the analyzer to refine TestType after the initial
// metadata-only inference pass.
func MergeContentInference(base, content InferResult) InferResult {
	if content.Type == TypeUnknown {
		return base
	}
	if base.Type == TypeUnknown {
		return content
	}
	if base.Type == content.Type {
		merged := base
		if content.Confidence > merged.Confidence {
			merged.Confidence = content.Confidence
		}
		merged.Evidence = append(merged.Evidence, content.Evidence...)
		return merged
	}
	// Disagreement: content-based wins, but evidence preserves the
	// path/suite signal so downstream consumers can see the conflict.
	merged := content
	merged.Evidence = append(merged.Evidence, base.Evidence...)
	merged.Evidence = append(merged.Evidence,
		"content-based integration detection overrode "+base.Type+" inference")
	if base.Confidence > merged.Confidence {
		// If the base inference was very confident, dampen the
		// content-based result slightly to reflect ambiguity.
		merged.Confidence *= 0.9
	}
	return merged
}
