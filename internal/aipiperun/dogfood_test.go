package aipiperun

import (
	"context"
	"testing"
	"time"

	"github.com/pmclSF/terrain/internal/aipipeline"
)

func TestRunRepo_TrainingDetectorTimebounded(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	mkfile(t, root, "pipelines/train.py",
		"import sagemaker\nfrom sklearn.ensemble import RandomForestClassifier\ndef train(X,y):\n    clf=RandomForestClassifier(); clf.fit(X,y); return clf\n")
	done := make(chan struct{})
	var findings []aipipeline.Finding
	var err error
	go func() {
		findings, err = RunRepo(context.Background(), root,
			[]string{"ai.train.missing_tracker"}, aipipeline.PostureObservability)
		close(done)
	}()
	select {
	case <-done:
		if err != nil { t.Fatalf("RunRepo error: %v", err) }
		t.Logf("got %d findings", len(findings))
		for _, f := range findings {
			t.Logf("  %s conf=%.3f atoms=%d", f.Path, f.Confidence, len(f.Atoms))
		}
	case <-time.After(5 * time.Second):
		t.Fatal("RunRepo did not return within 5 seconds")
	}
}
