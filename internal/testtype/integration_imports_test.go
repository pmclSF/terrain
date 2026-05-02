package testtype

import (
	"strings"
	"testing"
)

func TestInferFromContent_Empty(t *testing.T) {
	t.Parallel()
	r := InferFromContent("")
	if r.Type != TypeUnknown {
		t.Errorf("empty content type = %q, want unknown", r.Type)
	}
}

func TestInferFromContent_SupertestRequire(t *testing.T) {
	t.Parallel()
	src := `const request = require('supertest');
const app = require('../app');
describe('GET /users', () => { it('returns 200', () => request(app).get('/users')); });`
	r := InferFromContent(src)
	if r.Type != TypeIntegration {
		t.Errorf("type = %q, want integration", r.Type)
	}
	if r.Confidence < 0.85 {
		t.Errorf("confidence = %f, want >= 0.85", r.Confidence)
	}
	if !hasEvidence(r, "supertest") {
		t.Errorf("evidence missing supertest: %v", r.Evidence)
	}
}

func TestInferFromContent_SupertestImport(t *testing.T) {
	t.Parallel()
	src := `import request from 'supertest';
import { app } from '../app';`
	r := InferFromContent(src)
	if r.Type != TypeIntegration {
		t.Errorf("type = %q, want integration", r.Type)
	}
}

func TestInferFromContent_GoHttptest(t *testing.T) {
	t.Parallel()
	src := `package handlers_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetUsers(t *testing.T) {
	srv := httptest.NewServer(handler())
	defer srv.Close()
}`
	r := InferFromContent(src)
	if r.Type != TypeIntegration {
		t.Errorf("type = %q, want integration", r.Type)
	}
	if !hasEvidence(r, "httptest") {
		t.Errorf("evidence missing httptest: %v", r.Evidence)
	}
}

func TestInferFromContent_PythonRequests(t *testing.T) {
	t.Parallel()
	src := `import pytest
import requests

def test_user_endpoint():
    r = requests.get("http://localhost:8080/users")
    assert r.status_code == 200`
	r := InferFromContent(src)
	if r.Type != TypeIntegration {
		t.Errorf("type = %q, want integration", r.Type)
	}
}

func TestInferFromContent_PythonHttpx(t *testing.T) {
	t.Parallel()
	src := `import pytest
from httpx import AsyncClient`
	r := InferFromContent(src)
	if r.Type != TypeIntegration {
		t.Errorf("type = %q, want integration", r.Type)
	}
}

func TestInferFromContent_JavaMockMvc(t *testing.T) {
	t.Parallel()
	src := `package com.example;

import org.springframework.test.web.servlet.MockMvc;

class UserControllerTest {
}`
	r := InferFromContent(src)
	if r.Type != TypeIntegration {
		t.Errorf("type = %q, want integration", r.Type)
	}
}

func TestInferFromContent_RubyRackTest(t *testing.T) {
	t.Parallel()
	src := `require 'rack/test'

class AppTest < Minitest::Test
  include Rack::Test::Methods
end`
	r := InferFromContent(src)
	if r.Type != TypeIntegration {
		t.Errorf("type = %q, want integration", r.Type)
	}
}

func TestInferFromContent_NoMatch_PureUnit(t *testing.T) {
	t.Parallel()
	src := `import { add } from './math';

describe('add', () => {
  it('adds two numbers', () => {
    expect(add(1, 2)).toBe(3);
  });
});`
	r := InferFromContent(src)
	if r.Type != TypeUnknown {
		t.Errorf("pure unit test type = %q, want unknown", r.Type)
	}
}

func TestInferFromContent_NoMatch_ProseMention(t *testing.T) {
	t.Parallel()
	// The word "supertest" appears in a comment; should NOT match
	// because patterns require quote/paren context.
	src := `// We considered using supertest here but settled on a unit-only design.
import { add } from './math';`
	r := InferFromContent(src)
	if r.Type != TypeUnknown {
		t.Errorf("prose mention type = %q, want unknown", r.Type)
	}
}

func TestInferFromContent_MultipleLibraries(t *testing.T) {
	t.Parallel()
	src := `const request = require('supertest');
const nock = require('nock');`
	r := InferFromContent(src)
	if r.Type != TypeIntegration {
		t.Errorf("type = %q, want integration", r.Type)
	}
	// Both libraries should be present in evidence.
	if !hasEvidence(r, "supertest") || !hasEvidence(r, "nock") {
		t.Errorf("expected both libraries in evidence: %v", r.Evidence)
	}
}

func TestInferFromContent_Testcontainers(t *testing.T) {
	t.Parallel()
	src := `import { GenericContainer } from 'testcontainers';
describe('postgres integration', () => {});`
	r := InferFromContent(src)
	if r.Type != TypeIntegration {
		t.Errorf("type = %q, want integration", r.Type)
	}
}

func TestMergeContentInference_BothUnknown(t *testing.T) {
	t.Parallel()
	r := MergeContentInference(InferResult{Type: TypeUnknown}, InferResult{Type: TypeUnknown})
	if r.Type != TypeUnknown {
		t.Errorf("type = %q, want unknown", r.Type)
	}
}

func TestMergeContentInference_BaseOnly(t *testing.T) {
	t.Parallel()
	base := InferResult{Type: TypeUnit, Confidence: 0.5, Evidence: []string{"path-based"}}
	r := MergeContentInference(base, InferResult{Type: TypeUnknown})
	if r.Type != TypeUnit {
		t.Errorf("type = %q, want unit (base preserved)", r.Type)
	}
}

func TestMergeContentInference_AgreeBoostsConfidence(t *testing.T) {
	t.Parallel()
	base := InferResult{Type: TypeIntegration, Confidence: 0.7, Evidence: []string{"path: integration/"}}
	content := InferResult{Type: TypeIntegration, Confidence: 0.9, Evidence: []string{"library: supertest"}}
	r := MergeContentInference(base, content)
	if r.Type != TypeIntegration {
		t.Errorf("type = %q, want integration", r.Type)
	}
	if r.Confidence < 0.9 {
		t.Errorf("confidence = %f, want >= 0.9 (max of agreeing signals)", r.Confidence)
	}
}

func TestMergeContentInference_DisagreementContentWins(t *testing.T) {
	t.Parallel()
	base := InferResult{Type: TypeUnit, Confidence: 0.5, Evidence: []string{"jest framework"}}
	content := InferResult{Type: TypeIntegration, Confidence: 0.9, Evidence: []string{"library: supertest"}}
	r := MergeContentInference(base, content)
	if r.Type != TypeIntegration {
		t.Errorf("type = %q, want integration (content overrides)", r.Type)
	}
	if !hasEvidence(r, "overrode") {
		t.Errorf("expected override evidence: %v", r.Evidence)
	}
}

func hasEvidence(r InferResult, substr string) bool {
	for _, e := range r.Evidence {
		if strings.Contains(e, substr) {
			return true
		}
	}
	return false
}
