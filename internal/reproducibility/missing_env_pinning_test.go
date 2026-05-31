package reproducibility

import "testing"

func TestDetectMissingEnvPinning_SubscriptFires(t *testing.T) {
	t.Parallel()
	src := []byte(`import os

def get_model():
    return os.environ["MODEL"]
`)
	sigs := DetectMissingEnvPinning(src, "evals/inference.py")
	if len(sigs) != 1 {
		t.Fatalf("signals = %d, want 1: %+v", len(sigs), sigs)
	}
	if sigs[0].Metadata["envVar"] != "MODEL" {
		t.Errorf("envVar = %v", sigs[0].Metadata["envVar"])
	}
}

func TestDetectMissingEnvPinning_GetWithoutDefault(t *testing.T) {
	t.Parallel()
	src := []byte(`import os

MODEL = os.environ.get("MODEL")
`)
	sigs := DetectMissingEnvPinning(src, "evals/config.py")
	if len(sigs) != 1 {
		t.Errorf("expected 1 signal, got %d: %+v", len(sigs), sigs)
	}
}

func TestDetectMissingEnvPinning_GetWithDefaultSuppressed(t *testing.T) {
	t.Parallel()
	src := []byte(`import os

MODEL = os.environ.get("MODEL", "gpt-4o-mini")
`)
	sigs := DetectMissingEnvPinning(src, "evals/config.py")
	if len(sigs) != 0 {
		t.Errorf("default should suppress, got %+v", sigs)
	}
}

func TestDetectMissingEnvPinning_GetenvWithDefault(t *testing.T) {
	t.Parallel()
	src := []byte(`import os

MODEL = os.getenv("MODEL", "claude-opus-4-7")
`)
	sigs := DetectMissingEnvPinning(src, "evals/config.py")
	if len(sigs) != 0 {
		t.Errorf("getenv with default should suppress, got %+v", sigs)
	}
}

func TestDetectMissingEnvPinning_KwargDefault(t *testing.T) {
	t.Parallel()
	src := []byte(`import os

MODEL = os.environ.get("MODEL", default="gpt-4o-mini")
`)
	sigs := DetectMissingEnvPinning(src, "evals/config.py")
	if len(sigs) != 0 {
		t.Errorf("default= kwarg should suppress, got %+v", sigs)
	}
}

func TestDetectMissingEnvPinning_NonEvalPathSkipped(t *testing.T) {
	t.Parallel()
	src := []byte(`import os
def f(): return os.environ["X"]
`)
	sigs := DetectMissingEnvPinning(src, "src/cli.py")
	if len(sigs) != 0 {
		t.Errorf("non-eval path should not fire, got %+v", sigs)
	}
}
