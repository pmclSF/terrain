package reproducibility

import "testing"

func TestDetectNoSeed_FiresOnUnseededNumpy(t *testing.T) {
	t.Parallel()
	src := []byte(`import numpy as np

def eval_model():
    samples = np.random.rand(100)
    return samples.mean()
`)
	sigs := DetectNoSeed(src, "evals/perf.py")
	if len(sigs) != 1 {
		t.Fatalf("signals = %d, want 1: %+v", len(sigs), sigs)
	}
	if sigs[0].Metadata["library"] != "numpy" {
		t.Errorf("library = %v, want numpy", sigs[0].Metadata["library"])
	}
}

func TestDetectNoSeed_SuppressedByPriorSeed(t *testing.T) {
	t.Parallel()
	src := []byte(`import numpy as np

np.random.seed(42)

def eval_model():
    return np.random.rand(100).mean()
`)
	sigs := DetectNoSeed(src, "evals/perf.py")
	if len(sigs) != 0 {
		t.Errorf("expected suppression after seed, got %+v", sigs)
	}
}

func TestDetectNoSeed_HFSetSeedCoversAll(t *testing.T) {
	t.Parallel()
	src := []byte(`from transformers import set_seed
import torch

set_seed(0)

def eval():
    return torch.rand(3, 3)
`)
	sigs := DetectNoSeed(src, "evals/llm.py")
	if len(sigs) != 0 {
		t.Errorf("transformers set_seed should cover torch, got %+v", sigs)
	}
}

func TestDetectNoSeed_NonEvalPathSkipped(t *testing.T) {
	t.Parallel()
	src := []byte(`import numpy as np
def f(): return np.random.rand(1)
`)
	sigs := DetectNoSeed(src, "src/util.py")
	if len(sigs) != 0 {
		t.Errorf("non-eval path should not fire, got %+v", sigs)
	}
}

func TestDetectNoSeed_NoStochasticCalls(t *testing.T) {
	t.Parallel()
	src := []byte(`import numpy as np
def f(arr): return np.mean(arr)
`)
	sigs := DetectNoSeed(src, "evals/perf.py")
	if len(sigs) != 0 {
		t.Errorf("no stochastic calls should not fire, got %+v", sigs)
	}
}

func TestDetectNoSeed_TorchSpecific(t *testing.T) {
	t.Parallel()
	src := []byte(`import torch
def eval(): return torch.randn(10)
`)
	sigs := DetectNoSeed(src, "training/model.py")
	if len(sigs) != 1 {
		t.Fatalf("got %d sigs: %+v", len(sigs), sigs)
	}
	if sigs[0].Metadata["library"] != "torch" {
		t.Errorf("library = %v, want torch", sigs[0].Metadata["library"])
	}
}

func TestDetectNoSeed_Empty(t *testing.T) {
	t.Parallel()
	if got := DetectNoSeed(nil, "evals/x.py"); got != nil {
		t.Errorf("nil src: %+v", got)
	}
}
