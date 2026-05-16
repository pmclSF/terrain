package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/pmclSF/terrain/internal/aidetect"
	"github.com/pmclSF/terrain/internal/analysis"
	"github.com/pmclSF/terrain/internal/models"
)

// runPrintNetwork prints the unified detection network for a repo:
// frameworks, surfaces, evals (when present), and the surface→eval
// coverage edges. Per PRODUCT.md §16, this is the "what did Terrain
// see" diagnostic that adopters reach for when surface inference
// looks wrong.
func runPrintNetwork(root string) error {
	abs, err := filepath.Abs(root)
	if err != nil {
		return err
	}

	fmt.Printf("Terrain — network for %s\n\n", abs)

	det := aidetect.Detect(abs)

	// Frameworks.
	fmt.Println("Frameworks:")
	if len(det.Frameworks) == 0 {
		fmt.Println("  (none)")
	} else {
		seen := map[string]string{}
		for _, fw := range det.Frameworks {
			if _, ok := seen[fw.Name]; !ok {
				seen[fw.Name] = fw.Source
			}
		}
		names := make([]string, 0, len(seen))
		for n := range seen {
			names = append(names, n)
		}
		sort.Strings(names)
		for _, n := range names {
			fmt.Printf("  - %s (via %s)\n", n, seen[n])
		}
	}
	fmt.Println()

	// Source-file walk + AI surface inference.
	var sourceFiles []string
	_ = filepath.Walk(abs, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		if info.IsDir() {
			base := filepath.Base(path)
			if base == ".git" || base == "node_modules" || base == "vendor" ||
				base == ".venv" || base == "venv" || base == "dist" ||
				base == "build" || base == ".terrain" {
				return filepath.SkipDir
			}
			return nil
		}
		rel, _ := filepath.Rel(abs, path)
		sourceFiles = append(sourceFiles, rel)
		return nil
	})

	surfaces := analysis.DetectExtraAISurfaces(abs, nil, nil, sourceFiles)

	fmt.Printf("Detected surfaces (%d):\n", len(surfaces))
	byKind := map[models.CodeSurfaceKind][]models.CodeSurface{}
	for _, s := range surfaces {
		byKind[s.Kind] = append(byKind[s.Kind], s)
	}
	kinds := make([]models.CodeSurfaceKind, 0, len(byKind))
	for k := range byKind {
		kinds = append(kinds, k)
	}
	sort.Slice(kinds, func(i, j int) bool { return kinds[i] < kinds[j] })
	for _, k := range kinds {
		fmt.Printf("  %s:\n", k)
		for _, s := range byKind[k] {
			fmt.Printf("    %s — %s\n", s.Name, s.Path)
		}
	}
	fmt.Println()

	// Call-site detection summary.
	if len(det.CallSites) > 0 {
		fmt.Printf("AST-resolved AI call sites (%d):\n", len(det.CallSites))
		bySDK := map[string]int{}
		for _, cs := range det.CallSites {
			bySDK[cs.SDK]++
		}
		sdks := make([]string, 0, len(bySDK))
		for s := range bySDK {
			sdks = append(sdks, s)
		}
		sort.Strings(sdks)
		for _, s := range sdks {
			fmt.Printf("  %s: %d\n", s, bySDK[s])
		}
		fmt.Println()
	}

	if len(det.PromptFiles) > 0 {
		fmt.Printf("Prompt files (%d): %v\n\n", len(det.PromptFiles), summarizeList(det.PromptFiles, 5))
	}
	if len(det.DatasetFiles) > 0 {
		fmt.Printf("Dataset files (%d): %v\n\n", len(det.DatasetFiles), summarizeList(det.DatasetFiles, 5))
	}
	if len(det.ModelFiles) > 0 {
		fmt.Printf("Model invocation files (%d): %v\n\n", len(det.ModelFiles), summarizeList(det.ModelFiles, 5))
	}
	if len(det.EvalConfigs) > 0 {
		fmt.Printf("Eval config files (%d): %v\n", len(det.EvalConfigs), summarizeList(det.EvalConfigs, 5))
	}

	return nil
}

func summarizeList(items []string, max int) []string {
	if len(items) <= max {
		return items
	}
	return append(items[:max], fmt.Sprintf("…and %d more", len(items)-max))
}
