package hygiene

import "testing"

func TestDetectModelFixtureUnpinned_HFNoRevision(t *testing.T) {
	t.Parallel()
	src := []byte(`from transformers import AutoModelForCausalLM
model = AutoModelForCausalLM.from_pretrained("mistralai/Mistral-7B-v0.1")
`)
	sigs := DetectModelFixtureUnpinned(src, "src/load.py")
	if len(sigs) != 1 {
		t.Fatalf("expected 1 signal for missing revision, got %d: %+v", len(sigs), sigs)
	}
	if sigs[0].Metadata["family"] != "huggingface" {
		t.Errorf("family = %v", sigs[0].Metadata["family"])
	}
}

func TestDetectModelFixtureUnpinned_HFRevisionPinned(t *testing.T) {
	t.Parallel()
	src := []byte(`from transformers import AutoModelForCausalLM
model = AutoModelForCausalLM.from_pretrained(
    "mistralai/Mistral-7B-v0.1",
    revision="abc1234def5678",
)
`)
	sigs := DetectModelFixtureUnpinned(src, "src/load.py")
	if len(sigs) != 0 {
		t.Errorf("revision=SHA should suppress, got %+v", sigs)
	}
}

func TestDetectModelFixtureUnpinned_HFRevisionMain(t *testing.T) {
	t.Parallel()
	src := []byte(`from transformers import AutoModel
m = AutoModel.from_pretrained("bert-base-uncased", revision="main")
`)
	sigs := DetectModelFixtureUnpinned(src, "src/load.py")
	if len(sigs) != 1 {
		t.Errorf("revision=main should still fire (branch head), got %+v", sigs)
	}
}

func TestDetectModelFixtureUnpinned_TorchLoad_Unpinned(t *testing.T) {
	t.Parallel()
	src := []byte(`import torch
m = torch.load("model.pt")
`)
	sigs := DetectModelFixtureUnpinned(src, "src/load.py")
	if len(sigs) != 1 {
		t.Errorf("plain torch.load should fire, got %+v", sigs)
	}
}

func TestDetectModelFixtureUnpinned_TorchLoad_VersionSuffix(t *testing.T) {
	t.Parallel()
	src := []byte(`import torch
m = torch.load("model_v3.0.pt")
`)
	sigs := DetectModelFixtureUnpinned(src, "src/load.py")
	if len(sigs) != 0 {
		t.Errorf("version-suffixed path should suppress, got %+v", sigs)
	}
}

func TestDetectModelFixtureUnpinned_TorchLoad_Safetensors(t *testing.T) {
	t.Parallel()
	src := []byte(`import torch
m = torch.load("weights.safetensors")
`)
	sigs := DetectModelFixtureUnpinned(src, "src/load.py")
	if len(sigs) != 0 {
		t.Errorf(".safetensors path should suppress, got %+v", sigs)
	}
}

func TestDetectModelFixtureUnpinned_JoblibUnpinned(t *testing.T) {
	t.Parallel()
	src := []byte(`import joblib
m = joblib.load("classifier.joblib")
`)
	sigs := DetectModelFixtureUnpinned(src, "src/load.py")
	if len(sigs) != 1 {
		t.Errorf("plain joblib path should fire, got %+v", sigs)
	}
}

func TestDetectModelFixtureUnpinned_HashedPath(t *testing.T) {
	t.Parallel()
	src := []byte(`import torch
m = torch.load("model_abcdef1234.pt")
`)
	sigs := DetectModelFixtureUnpinned(src, "src/load.py")
	if len(sigs) != 0 {
		t.Errorf("hex-suffixed path should suppress, got %+v", sigs)
	}
}

func TestDetectModelFixtureUnpinned_NonModelCall(t *testing.T) {
	t.Parallel()
	src := []byte(`import json
def f(p): return json.load(open(p))
`)
	sigs := DetectModelFixtureUnpinned(src, "src/x.py")
	if len(sigs) != 0 {
		t.Errorf("non-model loader should not fire, got %+v", sigs)
	}
}

func TestLooksLikePinnedPath(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in   string
		want bool
	}{
		{"model.pt", false},
		{"model_v3.pt", false},
		{"model_v3.0.pt", true},
		{"model_v1.2.3.pt", true},
		{"weights_abcdef12.pt", true},
		{"weights_12345.pt", false}, // too short
		{"weights.safetensors", true},
		{"foo/bar/baz_a1b2c3d4.bin", true},
	}
	for _, c := range cases {
		if got := looksLikePinnedPath(c.in); got != c.want {
			t.Errorf("looksLikePinnedPath(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}
