package hygiene

import "testing"

func TestDetectEvalNoAssertion_FiresOnEmptyEvalTest(t *testing.T) {
	t.Parallel()
	src := []byte(`def test_summarize_quality():
    response = call_model("summarize this")
    print(response)
`)
	sigs := DetectEvalNoAssertion(src, "evals/summarize_test.py")
	if len(sigs) != 1 {
		t.Fatalf("signals = %d, want 1: %+v", len(sigs), sigs)
	}
	if sigs[0].Location.Symbol != "test_summarize_quality" {
		t.Errorf("fired on %q", sigs[0].Location.Symbol)
	}
}

func TestDetectEvalNoAssertion_SuppressedByAssertStmt(t *testing.T) {
	t.Parallel()
	src := []byte(`def test_summarize_quality():
    response = call_model("summarize this")
    assert response.score > 0.8
`)
	sigs := DetectEvalNoAssertion(src, "evals/summarize_test.py")
	if len(sigs) != 0 {
		t.Errorf("assert statement should suppress, got %+v", sigs)
	}
}

func TestDetectEvalNoAssertion_SuppressedByMetricCall(t *testing.T) {
	t.Parallel()
	src := []byte(`def test_factuality():
    response = call_model("...")
    evaluator.score(response, expected="...")
`)
	sigs := DetectEvalNoAssertion(src, "evaluations/factuality.py")
	if len(sigs) != 0 {
		t.Errorf("metric call should suppress, got %+v", sigs)
	}
}

func TestDetectEvalNoAssertion_NonEvalPathSkipped(t *testing.T) {
	t.Parallel()
	// Same shape but lives outside an eval dir — shouldn't fire.
	src := []byte(`def test_thing():
    pass
`)
	sigs := DetectEvalNoAssertion(src, "tests/api/test_users.py")
	if len(sigs) != 0 {
		t.Errorf("non-eval path should not fire, got %+v", sigs)
	}
}

func TestDetectEvalNoAssertion_PytestStyleAssertions(t *testing.T) {
	t.Parallel()
	cases := []struct {
		body string
		want bool // want signal?
	}{
		{"    pass\n", true},
		{"    self.assertEqual(a, b)\n", false},
		{"    pytest.fail('nope')\n", false},
		{"    expect(response).toEqual({})\n", false},
		{"    deepeval.assert_test_case(case, metrics=[])\n", false},
		{"    print('done')\n", true},
	}
	for _, c := range cases {
		src := []byte("def test_foo():\n" + c.body)
		sigs := DetectEvalNoAssertion(src, "evals/foo.py")
		got := len(sigs) > 0
		if got != c.want {
			t.Errorf("body %q: signal=%v, want %v", c.body, got, c.want)
		}
	}
}

func TestDetectEvalNoAssertion_EmptyInput(t *testing.T) {
	t.Parallel()
	if got := DetectEvalNoAssertion(nil, "evals/x.py"); got != nil {
		t.Errorf("nil src: %+v", got)
	}
	if got := DetectEvalNoAssertion([]byte("def test_x(): pass"), "tests/x.py"); got != nil {
		t.Errorf("non-eval path: %+v", got)
	}
}
