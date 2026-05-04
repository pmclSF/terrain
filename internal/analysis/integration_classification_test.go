package analysis

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/pmclSF/terrain/internal/models"
	"github.com/pmclSF/terrain/internal/testtype"
)

// TestRefineIntegrationClassification_PromotesUnitToIntegration verifies
// that a test file initially classified as TypeUnit (via metadata) gets
// promoted to TypeIntegration when its content imports supertest. This
// is the common shape Track 3.3 was designed to fix.
func TestRefineIntegrationClassification_PromotesUnitToIntegration(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	rel := "test/api.test.js"
	abs := filepath.Join(tmp, rel)
	mustWrite(t, abs, `const request = require('supertest');
const app = require('../app');
describe('GET /users', () => { it('200', () => request(app).get('/users')); });`)

	fc := NewFileCache(tmp)
	if _, ok := fc.ReadFile(rel); !ok {
		t.Fatalf("file cache could not read %s", rel)
	}

	cases := []models.TestCase{
		{
			FilePath:           rel,
			Framework:          "jest",
			TestName:           "GET /users",
			TestType:           testtype.TypeUnit,
			TestTypeConfidence: 0.5,
			TestTypeEvidence:   []string{"jest framework"},
		},
	}

	out := refineIntegrationClassification(context.Background(), cases, fc)
	if out[0].TestType != testtype.TypeIntegration {
		t.Errorf("TestType = %q, want integration (content override)", out[0].TestType)
	}
	if out[0].TestTypeConfidence < 0.7 {
		t.Errorf("Confidence = %f, want >= 0.7 after promotion", out[0].TestTypeConfidence)
	}
}

// TestRefineIntegrationClassification_LeavesPureUnitAlone verifies the
// false-positive guard: a unit test that doesn't import any integration
// library stays unit-classified.
func TestRefineIntegrationClassification_LeavesPureUnitAlone(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	rel := "src/math.test.js"
	abs := filepath.Join(tmp, rel)
	mustWrite(t, abs, `import { add } from './math';
describe('add', () => { it('adds', () => expect(add(1,2)).toBe(3)); });`)

	fc := NewFileCache(tmp)
	cases := []models.TestCase{
		{
			FilePath:           rel,
			Framework:          "jest",
			TestType:           testtype.TypeUnit,
			TestTypeConfidence: 0.5,
		},
	}

	out := refineIntegrationClassification(context.Background(), cases, fc)
	if out[0].TestType != testtype.TypeUnit {
		t.Errorf("TestType = %q, want unit (no override)", out[0].TestType)
	}
}

// TestRefineIntegrationClassification_GoHttptest verifies the Go path:
// a Go test file that imports net/http/httptest gets promoted.
func TestRefineIntegrationClassification_GoHttptest(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	rel := "handlers/users_test.go"
	abs := filepath.Join(tmp, rel)
	mustWrite(t, abs, `package handlers_test

import (
	"net/http/httptest"
	"testing"
)

func TestGetUsers(t *testing.T) {
	srv := httptest.NewServer(handler())
	defer srv.Close()
}`)

	fc := NewFileCache(tmp)
	cases := []models.TestCase{
		{
			FilePath:           rel,
			Framework:          "go-testing",
			TestType:           testtype.TypeUnit,
			TestTypeConfidence: 0.5,
		},
	}

	out := refineIntegrationClassification(context.Background(), cases, fc)
	if out[0].TestType != testtype.TypeIntegration {
		t.Errorf("TestType = %q, want integration", out[0].TestType)
	}
}

// TestRefineIntegrationClassification_NilCacheReturnsInput verifies the
// nil-cache early return — important for unit tests that build a
// snapshot without a populated FileCache.
func TestRefineIntegrationClassification_NilCacheReturnsInput(t *testing.T) {
	t.Parallel()
	cases := []models.TestCase{{FilePath: "x", TestType: testtype.TypeUnit}}
	out := refineIntegrationClassification(context.Background(), cases, nil)
	if len(out) != 1 || out[0].TestType != testtype.TypeUnit {
		t.Errorf("nil cache should leave cases unchanged")
	}
}

// TestRefineIntegrationClassification_RespectsCancellation verifies that
// a cancelled context returns early without panicking. The function
// only checks ctx every 64 cases, so we feed it 100 to trigger the
// check.
func TestRefineIntegrationClassification_RespectsCancellation(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	fc := NewFileCache(tmp)

	cases := make([]models.TestCase, 100)
	for i := range cases {
		cases[i] = models.TestCase{FilePath: "x", TestType: testtype.TypeUnit}
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	// Should not panic and should return the input slice intact-ish.
	out := refineIntegrationClassification(ctx, cases, fc)
	if len(out) != 100 {
		t.Errorf("len(out) = %d, want 100", len(out))
	}
}

func mustWrite(t *testing.T, abs, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", abs, err)
	}
	if err := os.WriteFile(abs, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", abs, err)
	}
}
