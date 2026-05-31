package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	conv "github.com/pmclSF/terrain/internal/convert"
)

const (
	defaultLegacyRef   = "466b3c8"
	defaultIterations  = 12
	defaultWarmup      = 2
	defaultThreshold   = 1.00
	suiteSourceRuntime = "source-runtime"
)

type benchmarkOptions struct {
	LegacyRef  string
	Iterations int
	Warmup     int
	Threshold  float64
	Only       string
	JSON       bool
}

type benchmarkCase struct {
	Name      string `json:"name"`
	From      string `json:"from"`
	To        string `json:"to"`
	Language  string `json:"language"`
	Category  string `json:"category"`
	Input     string `json:"-"`
	InputSize int    `json:"inputSizeBytes"`
}

type sampleStats struct {
	SamplesNs []int64 `json:"samplesNs"`
	MinNs     int64   `json:"minNs"`
	MedianNs  int64   `json:"medianNs"`
	P95Ns     int64   `json:"p95Ns"`
	MaxNs     int64   `json:"maxNs"`
	MeanNs    int64   `json:"meanNs"`
}

type benchmarkCaseResult struct {
	Case        benchmarkCase `json:"case"`
	GoCurrent   sampleStats   `json:"goCurrent"`
	LegacyJS    sampleStats   `json:"legacyJs"`
	MedianRatio float64       `json:"medianRatio"`
	MeanRatio   float64       `json:"meanRatio"`
	Threshold   float64       `json:"threshold"`
	WithinFloor bool          `json:"withinFloor"`
	DeltaMedian int64         `json:"deltaMedianNs"`
	DeltaMean   int64         `json:"deltaMeanNs"`
}

type benchmarkReport struct {
	Suite             string                `json:"suite"`
	LegacyRef         string                `json:"legacyRef"`
	Iterations        int                   `json:"iterations"`
	Warmup            int                   `json:"warmup"`
	Threshold         float64               `json:"threshold"`
	GeneratedAt       string                `json:"generatedAt"`
	CaseCount         int                   `json:"caseCount"`
	RegressionCount   int                   `json:"regressionCount"`
	AllWithinFloor    bool                  `json:"allWithinFloor"`
	LegacyNodeVersion string                `json:"legacyNodeVersion,omitempty"`
	Results           []benchmarkCaseResult `json:"results"`
}

func main() {
	opts := parseFlags(os.Args[1:])
	report, err := runBenchmark(opts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	if opts.JSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(report)
	} else {
		printReport(report)
	}

	if !report.AllWithinFloor {
		os.Exit(1)
	}
}

func parseFlags(args []string) benchmarkOptions {
	fs := flag.NewFlagSet("terrain-convert-bench", flag.ExitOnError)
	var opts benchmarkOptions
	fs.StringVar(&opts.LegacyRef, "legacy-ref", defaultLegacyRef, "git ref for the legacy JS converter runtime")
	fs.IntVar(&opts.Iterations, "iterations", defaultIterations, "timed iterations per conversion direction")
	fs.IntVar(&opts.Warmup, "warmup", defaultWarmup, "warmup iterations per conversion direction")
	fs.Float64Var(&opts.Threshold, "threshold", defaultThreshold, "maximum allowed Go/JS runtime ratio before flagging regression")
	fs.StringVar(&opts.Only, "only", "", "comma-separated direction filters, for example jest-vitest,cypress-playwright")
	fs.BoolVar(&opts.JSON, "json", false, "emit JSON")
	_ = fs.Parse(args)
	return opts
}

func runBenchmark(opts benchmarkOptions) (benchmarkReport, error) {
	if opts.Iterations <= 0 {
		return benchmarkReport{}, fmt.Errorf("--iterations must be > 0")
	}
	if opts.Warmup < 0 {
		return benchmarkReport{}, fmt.Errorf("--warmup must be >= 0")
	}
	if opts.Threshold <= 0 {
		return benchmarkReport{}, fmt.Errorf("--threshold must be > 0")
	}

	cases, err := buildBenchmarkCases(opts.Only)
	if err != nil {
		return benchmarkReport{}, err
	}
	if len(cases) == 0 {
		return benchmarkReport{}, fmt.Errorf("no benchmark cases selected")
	}

	repoRoot, err := os.Getwd()
	if err != nil {
		return benchmarkReport{}, fmt.Errorf("resolve working directory: %w", err)
	}

	legacyRoot, err := prepareLegacyRuntime(repoRoot, opts.LegacyRef)
	if err != nil {
		return benchmarkReport{}, err
	}
	defer os.RemoveAll(legacyRoot)

	nodeVersion, _ := runCommandCapture(legacyRoot, "node", "--version")

	results := make([]benchmarkCaseResult, 0, len(cases))
	regressions := 0
	for _, item := range cases {
		goStats, err := benchmarkCurrentGo(item, opts.Warmup, opts.Iterations)
		if err != nil {
			return benchmarkReport{}, fmt.Errorf("benchmark Go %s: %w", item.Name, err)
		}
		legacyStats, err := benchmarkLegacyJS(legacyRoot, item, opts.Warmup, opts.Iterations)
		if err != nil {
			return benchmarkReport{}, fmt.Errorf("benchmark legacy JS %s: %w", item.Name, err)
		}

		medianRatio := ratio(goStats.MedianNs, legacyStats.MedianNs)
		meanRatio := ratio(goStats.MeanNs, legacyStats.MeanNs)
		withinFloor := medianRatio <= opts.Threshold
		if !withinFloor {
			regressions++
		}

		results = append(results, benchmarkCaseResult{
			Case:        item,
			GoCurrent:   goStats,
			LegacyJS:    legacyStats,
			MedianRatio: medianRatio,
			MeanRatio:   meanRatio,
			Threshold:   opts.Threshold,
			WithinFloor: withinFloor,
			DeltaMedian: goStats.MedianNs - legacyStats.MedianNs,
			DeltaMean:   goStats.MeanNs - legacyStats.MeanNs,
		})
	}

	return benchmarkReport{
		Suite:             suiteSourceRuntime,
		LegacyRef:         opts.LegacyRef,
		Iterations:        opts.Iterations,
		Warmup:            opts.Warmup,
		Threshold:         opts.Threshold,
		GeneratedAt:       time.Now().UTC().Format(time.RFC3339),
		CaseCount:         len(results),
		RegressionCount:   regressions,
		AllWithinFloor:    regressions == 0,
		LegacyNodeVersion: strings.TrimSpace(nodeVersion),
		Results:           results,
	}, nil
}

func buildBenchmarkCases(filter string) ([]benchmarkCase, error) {
	filters := make([]string, 0, 4)
	for _, item := range strings.Split(filter, ",") {
		item = strings.ToLower(strings.TrimSpace(item))
		if item != "" {
			filters = append(filters, item)
		}
	}

	cases := make([]benchmarkCase, 0, len(conv.SupportedDirections()))
	for _, direction := range conv.SupportedDirections() {
		name := direction.From + "-" + direction.To
		if len(filters) > 0 && !matchesAnyFilter(name, filters) {
			continue
		}

		input, ok := sourceFixture(direction.From)
		if !ok {
			return nil, fmt.Errorf("missing source fixture for %s", direction.From)
		}

		cases = append(cases, benchmarkCase{
			Name:      name,
			From:      direction.From,
			To:        direction.To,
			Language:  direction.Language,
			Category:  direction.Category,
			Input:     input,
			InputSize: len(input),
		})
	}

	sort.Slice(cases, func(i, j int) bool { return cases[i].Name < cases[j].Name })
	return cases, nil
}

func matchesAnyFilter(name string, filters []string) bool {
	for _, filter := range filters {
		if strings.Contains(name, filter) {
			return true
		}
	}
	return false
}

func benchmarkCurrentGo(item benchmarkCase, warmup, iterations int) (sampleStats, error) {
	direction, ok := conv.LookupDirection(item.From, item.To)
	if !ok {
		return sampleStats{}, fmt.Errorf("unsupported direction: %s", item.Name)
	}

	for i := 0; i < warmup; i++ {
		if _, err := conv.ConvertSource(direction, item.Input); err != nil {
			return sampleStats{}, err
		}
	}

	samples := make([]int64, 0, iterations)
	for i := 0; i < iterations; i++ {
		start := time.Now()
		if _, err := conv.ConvertSource(direction, item.Input); err != nil {
			return sampleStats{}, err
		}
		samples = append(samples, time.Since(start).Nanoseconds())
	}
	return summarizeSamples(samples), nil
}

func benchmarkLegacyJS(legacyRoot string, item benchmarkCase, warmup, iterations int) (sampleStats, error) {
	inputPath := filepath.Join(legacyRoot, "bench-input.txt")
	if err := os.WriteFile(inputPath, []byte(item.Input), 0o644); err != nil {
		return sampleStats{}, fmt.Errorf("write benchmark input: %w", err)
	}

	output, err := runCommandCapture(
		legacyRoot,
		"node",
		filepath.Join(legacyRoot, "bench.mjs"),
		item.From,
		item.To,
		inputPath,
		fmt.Sprintf("%d", warmup),
		fmt.Sprintf("%d", iterations),
	)
	if err != nil {
		return sampleStats{}, err
	}

	var payload struct {
		SamplesNs []int64 `json:"samplesNs"`
	}
	if err := json.Unmarshal([]byte(output), &payload); err != nil {
		return sampleStats{}, fmt.Errorf("decode legacy benchmark output: %w", err)
	}
	if len(payload.SamplesNs) == 0 {
		return sampleStats{}, errors.New("legacy benchmark returned no samples")
	}
	return summarizeSamples(payload.SamplesNs), nil
}

func summarizeSamples(samples []int64) sampleStats {
	items := append([]int64(nil), samples...)
	sort.Slice(items, func(i, j int) bool { return items[i] < items[j] })
	var total int64
	for _, item := range items {
		total += item
	}

	return sampleStats{
		SamplesNs: items,
		MinNs:     items[0],
		MedianNs:  percentile(items, 0.50),
		P95Ns:     percentile(items, 0.95),
		MaxNs:     items[len(items)-1],
		MeanNs:    total / int64(len(items)),
	}
}

func percentile(sorted []int64, p float64) int64 {
	if len(sorted) == 0 {
		return 0
	}
	if len(sorted) == 1 {
		return sorted[0]
	}
	index := int(math.Ceil((float64(len(sorted)) * p) - 1.0))
	if index < 0 {
		index = 0
	}
	if index >= len(sorted) {
		index = len(sorted) - 1
	}
	return sorted[index]
}

func ratio(current, baseline int64) float64 {
	if baseline <= 0 {
		return 0
	}
	return float64(current) / float64(baseline)
}

func prepareLegacyRuntime(repoRoot, ref string) (string, error) {
	root, err := os.MkdirTemp(repoRoot, ".terrain-convert-bench-*")
	if err != nil {
		return "", fmt.Errorf("create legacy runtime dir: %w", err)
	}

	if err := extractGitPaths(repoRoot, ref, root, "package.json", "package-lock.json", "src"); err != nil {
		_ = os.RemoveAll(root)
		return "", fmt.Errorf("extract legacy runtime: %w", err)
	}

	if err := os.WriteFile(filepath.Join(root, "bench.mjs"), []byte(legacyBenchScript), 0o644); err != nil {
		_ = os.RemoveAll(root)
		return "", fmt.Errorf("write legacy benchmark script: %w", err)
	}

	if _, err := os.Stat(filepath.Join(root, "node_modules", "@babel", "parser", "package.json")); errors.Is(err, os.ErrNotExist) {
		if _, err := runCommandCapture(root, "npm", "ci", "--ignore-scripts"); err != nil {
			_ = os.RemoveAll(root)
			return "", fmt.Errorf("install legacy benchmark dependencies: %w", err)
		}
	}

	return root, nil
}

func extractGitPaths(repoRoot, ref, target string, paths ...string) error {
	archiveCmd := exec.Command("git", append([]string{"archive", ref}, paths...)...)
	archiveCmd.Dir = repoRoot

	tarCmd := exec.Command("tar", "-xf", "-", "-C", target)
	pr, pw := io.Pipe()
	archiveCmd.Stdout = pw
	archiveCmd.Stderr = os.Stderr
	tarCmd.Stdin = pr
	tarCmd.Stderr = os.Stderr

	if err := tarCmd.Start(); err != nil {
		return err
	}
	archiveErr := archiveCmd.Run()
	_ = pw.Close()
	tarErr := tarCmd.Wait()
	if archiveErr != nil {
		return archiveErr
	}
	return tarErr
}

func runCommandCapture(dir string, name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return "", fmt.Errorf("%s %s: %s", name, strings.Join(args, " "), msg)
	}
	return stdout.String(), nil
}

func printReport(report benchmarkReport) {
	fmt.Printf("Terrain Converter Benchmark\n\n")
	fmt.Printf("  Suite: %s\n", report.Suite)
	fmt.Printf("  Legacy ref: %s\n", report.LegacyRef)
	fmt.Printf("  Node: %s\n", emptyFallback(report.LegacyNodeVersion, "unknown"))
	fmt.Printf("  Iterations: %d\n", report.Iterations)
	fmt.Printf("  Warmup: %d\n", report.Warmup)
	fmt.Printf("  Threshold: %.2fx\n", report.Threshold)
	fmt.Printf("  Cases: %d\n", report.CaseCount)
	fmt.Printf("  Regressions: %d\n", report.RegressionCount)
	fmt.Println()

	for _, result := range report.Results {
		status := "OK"
		if !result.WithinFloor {
			status = "REGRESSION"
		}
		fmt.Printf("[%s] %s\n", status, result.Case.Name)
		fmt.Printf("  median: Go %s | JS %s | ratio %.2fx\n", humanDuration(result.GoCurrent.MedianNs), humanDuration(result.LegacyJS.MedianNs), result.MedianRatio)
		fmt.Printf("  p95:    Go %s | JS %s\n", humanDuration(result.GoCurrent.P95Ns), humanDuration(result.LegacyJS.P95Ns))
		fmt.Printf("  mean:   Go %s | JS %s | ratio %.2fx\n", humanDuration(result.GoCurrent.MeanNs), humanDuration(result.LegacyJS.MeanNs), result.MeanRatio)
	}
}

func humanDuration(ns int64) string {
	return time.Duration(ns).String()
}

func emptyFallback(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func sourceFixture(framework string) (string, bool) {
	fixture, ok := sourceFixtures[strings.ToLower(strings.TrimSpace(framework))]
	return fixture, ok
}

var sourceFixtures = map[string]string{
	"cypress": `describe('test', () => {
  it('works', () => {
    cy.visit('/');
    cy.get('#btn').click();
  });
});
`,
	"playwright": `import { test, expect } from '@playwright/test';
test.describe('test', () => {
  test('works', async ({ page }) => {
    await page.goto('/');
    await page.locator('#btn').click();
  });
});
`,
	"selenium": `const { Builder, By } = require('selenium-webdriver');
describe('test', () => {
  it('works', async () => {
    const driver = await new Builder().forBrowser('chrome').build();
    await driver.get('http://localhost');
    await driver.findElement(By.css('#btn')).click();
  });
});
`,
	"jest": `describe('test', () => {
  it('works', () => {
    expect(1 + 1).toBe(2);
  });
});
`,
	"vitest": `import { describe, it, expect } from 'vitest';
describe('test', () => {
  it('works', () => {
    expect(1 + 1).toBe(2);
  });
});
`,
	"mocha": `const { expect } = require('chai');
describe('test', () => {
  it('works', () => {
    expect(1 + 1).to.equal(2);
  });
});
`,
	"jasmine": `describe('test', () => {
  it('works', () => {
    const spy = jasmine.createSpy();
    spy();
    expect(spy).toHaveBeenCalled();
  });
});
`,
	"webdriverio": `describe('test', () => {
  it('works', async () => {
    await browser.url('/');
    await $('#btn').click();
  });
});
`,
	"puppeteer": `const puppeteer = require('puppeteer');
describe('test', () => {
  it('works', async () => {
    const browser = await puppeteer.launch();
    const page = await browser.newPage();
    await page.goto('/');
    await page.click('#btn');
    await browser.close();
  });
});
`,
	"testcafe": "import { Selector } from 'testcafe';\n" +
		"fixture`Test`.page`http://localhost`;\n" +
		"test('works', async t => {\n" +
		"  await t.click('#btn');\n" +
		"});\n",
	"junit4": `import org.junit.Test;
import static org.junit.Assert.*;

public class MyTest {
    @Test
    public void testBasic() {
        assertEquals(1, 1);
    }
}
`,
	"junit5": `import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.Assertions;

public class MyTest {
    @Test
    public void testBasic() {
        Assertions.assertEquals(1, 1);
    }
}
`,
	"testng": `import org.testng.annotations.Test;
import org.testng.Assert;

public class MyTest {
    @Test
    public void testBasic() {
        Assert.assertEquals(1, 1);
    }
}
`,
	"pytest": `import pytest

def test_basic():
    assert 1 == 1
`,
	"unittest": `import unittest

class TestBasic(unittest.TestCase):
    def test_basic(self):
        self.assertEqual(1, 1)
`,
	"nose2": `from nose.tools import assert_equal

def test_basic():
    assert_equal(1, 1)
`,
}

const legacyBenchScript = `import fs from 'fs/promises';
import { ConverterFactory } from './src/core/ConverterFactory.js';

const [, , from, to, inputPath, warmupArg, iterationsArg] = process.argv;
const warmup = Number.parseInt(warmupArg, 10);
const iterations = Number.parseInt(iterationsArg, 10);
const input = await fs.readFile(inputPath, 'utf8');
const converter = await ConverterFactory.createConverter(from, to);

for (let i = 0; i < warmup; i++) {
  await converter.convert(input);
}

const samplesNs = [];
for (let i = 0; i < iterations; i++) {
  const start = process.hrtime.bigint();
  await converter.convert(input);
  const end = process.hrtime.bigint();
  samplesNs.push(Number(end - start));
}

process.stdout.write(JSON.stringify({ samplesNs }));
`
