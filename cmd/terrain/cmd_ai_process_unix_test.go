//go:build unix

package main

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRunEvalCommand_TimeoutKillsProcessGroup(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	marker := filepath.Join(dir, "left-behind")
	var stderr bytes.Buffer
	err := runEvalCommand(dir, []string{
		os.Args[0],
		"-test.run=^TestRunEvalCommand_HelperSpawnMarker$",
		"--",
		"terrain-helper-spawn-marker",
		marker,
	}, true, 1500*time.Millisecond, &stderr)
	if err == nil {
		t.Fatal("expected timeout error")
	}
	if !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("error should mention timeout, got %v", err)
	}

	time.Sleep(3 * time.Second)
	if _, statErr := os.Stat(marker); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("timeout should kill descendant eval processes; marker stat err=%v", statErr)
	}
}

func TestRunEvalCommand_HelperSpawnMarker(t *testing.T) {
	marker, ok := helperValueAfter("terrain-helper-spawn-marker")
	if !ok {
		t.Skip("helper process only")
	}
	cmd := exec.Command(os.Args[0],
		"-test.run=^TestRunEvalCommand_HelperWriteMarker$",
		"--",
		"terrain-helper-write-marker",
		marker,
	)
	if err := cmd.Start(); err != nil {
		t.Fatalf("start marker child: %v", err)
	}
	time.Sleep(10 * time.Second)
}

func TestRunEvalCommand_HelperWriteMarker(t *testing.T) {
	marker, ok := helperValueAfter("terrain-helper-write-marker")
	if !ok {
		t.Skip("helper process only")
	}
	time.Sleep(2500 * time.Millisecond)
	if err := os.WriteFile(marker, []byte("survived"), 0o644); err != nil {
		t.Fatalf("write marker: %v", err)
	}
}

func helperValueAfter(marker string) (string, bool) {
	for i, arg := range os.Args {
		if arg == marker && i+1 < len(os.Args) {
			return os.Args[i+1], true
		}
	}
	return "", false
}
