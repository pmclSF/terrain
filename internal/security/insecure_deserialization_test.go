package security

import "testing"

func TestDetectInsecureDeserialization_PickleLoad(t *testing.T) {
	t.Parallel()
	src := []byte(`import pickle

def load_model(path):
    with open(path, "rb") as f:
        return pickle.load(f)
`)
	sigs := DetectInsecureDeserialization(src, "src/loader.py")
	if len(sigs) != 1 {
		t.Fatalf("signals = %d, want 1: %+v", len(sigs), sigs)
	}
	if sigs[0].Metadata["primitive"] != "pickle.load" {
		t.Errorf("primitive = %v", sigs[0].Metadata["primitive"])
	}
}

func TestDetectInsecureDeserialization_TorchLoad_Unsafe(t *testing.T) {
	t.Parallel()
	src := []byte(`import torch

def load_model(path):
    return torch.load(path)
`)
	sigs := DetectInsecureDeserialization(src, "src/model.py")
	if len(sigs) != 1 {
		t.Fatalf("expected unsafe torch.load to fire: %+v", sigs)
	}
}

func TestDetectInsecureDeserialization_TorchLoad_Safe(t *testing.T) {
	t.Parallel()
	src := []byte(`import torch

def load_model(path):
    return torch.load(path, weights_only=True)
`)
	sigs := DetectInsecureDeserialization(src, "src/model.py")
	if len(sigs) != 0 {
		t.Errorf("torch.load with weights_only=True should be suppressed, got %+v", sigs)
	}
}

func TestDetectInsecureDeserialization_YAMLLoad_Unsafe(t *testing.T) {
	t.Parallel()
	src := []byte(`import yaml

def load_config(path):
    return yaml.load(open(path))
`)
	sigs := DetectInsecureDeserialization(src, "config.py")
	if len(sigs) != 1 {
		t.Fatalf("expected unsafe yaml.load to fire: %+v", sigs)
	}
}

func TestDetectInsecureDeserialization_YAMLLoad_SafeLoader(t *testing.T) {
	t.Parallel()
	src := []byte(`import yaml

def load_config(path):
    return yaml.load(open(path), Loader=yaml.SafeLoader)
`)
	sigs := DetectInsecureDeserialization(src, "config.py")
	if len(sigs) != 0 {
		t.Errorf("Loader=SafeLoader should suppress, got %+v", sigs)
	}
}

func TestDetectInsecureDeserialization_Joblib(t *testing.T) {
	t.Parallel()
	src := []byte(`import joblib

def load_estimator(path):
    return joblib.load(path)
`)
	sigs := DetectInsecureDeserialization(src, "src/load.py")
	if len(sigs) != 1 {
		t.Fatalf("joblib.load should fire: %+v", sigs)
	}
	if sigs[0].Severity != "critical" {
		t.Errorf("severity = %q", sigs[0].Severity)
	}
}

func TestDetectInsecureDeserialization_MultipleSites(t *testing.T) {
	t.Parallel()
	src := []byte(`import pickle, dill, marshal

def a(p): return pickle.load(p)
def b(p): return dill.loads(p)
def c(p): return marshal.load(p)
`)
	sigs := DetectInsecureDeserialization(src, "src/multi.py")
	if len(sigs) != 3 {
		t.Errorf("expected 3 signals (one per call), got %d", len(sigs))
	}
}

func TestDetectInsecureDeserialization_NonDeserializationCalls(t *testing.T) {
	t.Parallel()
	src := []byte(`import json
def f(p): return json.load(open(p))  # json.load is safe
`)
	sigs := DetectInsecureDeserialization(src, "src/x.py")
	if len(sigs) != 0 {
		t.Errorf("json.load should not fire, got %+v", sigs)
	}
}
