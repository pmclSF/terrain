# Terrain — signal-first test intelligence platform

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
DATE    ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS := -s -w -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)
GO_OWNED_PKGS := ./cmd/... ./internal/...

.PHONY: build test lint clean demo benchmark-fetch benchmark-smoke benchmark-full benchmark-stress benchmark-summary benchmark-convert install docs-linkcheck \
       test-golden test-determinism test-schema test-adversarial test-e2e test-cli test-bench golden-update pr-gate release-gate \
       sbom sbom-cyclonedx sbom-spdx release-dry-run go-release-verify js-release-verify extension-verify release-verify \
       docs-gen docs-verify calibrate bench-baseline bench-gate memory-bench truth-verify

# Build the CLI binary
build:
	go build -ldflags "$(LDFLAGS)" -o terrain ./cmd/terrain

# Install to $GOPATH/bin
install:
	go install -ldflags "$(LDFLAGS)" ./cmd/terrain

# Run all Go tests
test:
	go test $(GO_OWNED_PKGS)

# Run tests with verbose output
test-v:
	go test -v $(GO_OWNED_PKGS)

# Run tests with coverage
test-cover:
	go test -coverprofile=coverage.out $(GO_OWNED_PKGS)
	go tool cover -func=coverage.out

# Build check (compile only, no binary output)
check:
	go vet $(GO_OWNED_PKGS)
	go build ./cmd/terrain

# Clean build artifacts
clean:
	rm -f terrain coverage.out terrain.cdx.json terrain.spdx.json

# Run demo: analyze the current repository
demo:
	@echo "=== Terrain Demo ==="
	@echo ""
	@echo "--- terrain analyze ---"
	go run ./cmd/terrain analyze
	@echo ""
	@echo "--- terrain summary ---"
	go run ./cmd/terrain summary
	@echo ""
	@echo "--- terrain metrics ---"
	go run ./cmd/terrain metrics

# ── Verification Layer Targets ──────────────────────────────

# Run golden tests only (fast)
test-golden:
	go test ./internal/testdata/ -run 'Golden' -count=1

# Run determinism tests only
test-determinism:
	go test ./internal/testdata/ -run 'Determinism' -count=1

# Run schema tests only
test-schema:
	go test ./internal/testdata/ -run 'Schema' -count=1

# Run adversarial tests only
test-adversarial:
	go test ./internal/testdata/ -run 'Adversarial' -count=1

# Run E2E scenario tests only
test-e2e:
	go test ./internal/testdata/ -run 'E2E' -count=1

# Run CLI regression tests only
test-cli:
	go test ./internal/testdata/ -run 'CLI' -count=1 -timeout 120s

# Run benchmarks
test-bench:
	go test -bench . -benchmem ./internal/testdata/ -run '^$$'

# Update golden files (review changes in git diff before committing)
golden-update:
	go test ./internal/testdata/ -run 'Golden' -update

# ── SBOM Generation ───────────────────────────────────────
# Requires: syft (https://github.com/anchore/syft)
#   brew install syft  OR  go install github.com/anchore/syft/cmd/syft@latest

# Generate CycloneDX SBOM from the built binary
sbom-cyclonedx: build
	syft terrain --output cyclonedx-json=terrain.cdx.json
	@echo "CycloneDX SBOM written to terrain.cdx.json"

# Generate SPDX SBOM from the built binary
sbom-spdx: build
	syft terrain --output spdx-json=terrain.spdx.json
	@echo "SPDX SBOM written to terrain.spdx.json"

# Generate both SBOM formats
sbom: sbom-cyclonedx sbom-spdx

# Dry-run GoReleaser to verify config and preview artifacts
release-dry-run:
	goreleaser release --snapshot --clean --skip=publish,sign

# ── Release Gates ──────────────────────────────────────────

# PR gate: fast checks required on every PR
pr-gate:
	$(MAKE) check
	$(MAKE) test

# Release gate: full verification required before release
release-gate: go-release-verify

go-release-verify:
	go vet $(GO_OWNED_PKGS)
	go test $(GO_OWNED_PKGS)
	go build ./cmd/terrain
	go test ./cmd/terrain/ -run TestSnapshot -count=1 -v

npm-release-verify:
	npm ci
	npm run release:verify

extension-verify:
	npm --prefix extension/vscode ci
	npm --prefix extension/vscode run compile
	npm --prefix extension/vscode test

# ── Generated documentation ─────────────────────────────────
# `docs-gen` rewrites docs/signals/manifest.json from
# internal/signals.allSignalManifest. `docs-verify` writes to a tempdir
# and diffs against the committed copy so CI fails when a manifest
# change ships without the regenerated docs.
docs-gen:
	go run ./cmd/terrain-docs-gen

docs-verify:
	@scripts/docs-verify.sh

# ── Parity gate ────────────────────────────────────────────
# Reads docs/release/parity/{rubric,scores}.yaml and emits the
# pillar-parity matrix + verdict. Exits non-zero when any pillar
# is below its hard-gate floor (Gate ≥ 4, Understand ≥ 3 in 0.2.0).
# Soft gates (Align in 0.2.0) print a WARN banner but do not fail.
# Source-of-truth doc is `docs/release/0.2.x-maturity-audit.md`.
pillar-parity:
	@go run ./cmd/terrain-parity-gate

# JSON form for CI integration / external tooling.
pillar-parity-json:
	@go run ./cmd/terrain-parity-gate --json

# Compact form: per-area + per-pillar floor map only.
pillar-parity-floor:
	@go run ./cmd/terrain-parity-gate --floor-map

# `docs-linkcheck` walks docs/ and verifies that every intra-repo
# markdown link resolves to a real file. Skips docs/internal/ and
# docs/legacy/ by default — those subtrees hold planning notes whose
# link discipline is inherited debt; run with -include-internal to
# also scan them. External links (http/https/mailto) are out of
# scope. Track 9.8 deliverable for the 0.2.0 parity plan.
docs-linkcheck:
	@go run ./cmd/terrain-docs-linkcheck

# `truth-verify` cross-checks docs/release/feature-status.md against
# the canonical signal manifest. Every signal name documented in the
# curated table must reference a real manifest entry; references that
# don't resolve (typo, renamed, removed) fail the build. Orphan stable
# signals (in the manifest, not in the curated doc) print as
# advisory warnings — pass --strict-orphans to fail on them too.
# Track 9.7 deliverable for the 0.2.0 parity plan.
truth-verify:
	@go run ./cmd/terrain-truth-verify

# ── Calibration corpus ──────────────────────────────────────
# Runs the engine pipeline against every fixture under tests/calibration/
# and prints precision/recall per detector. Today a smoke gate (advisory
# misses); flips to a hard ≥90% precision gate once the corpus is
# populated. See docs/calibration/CORPUS.md.
calibrate:
	go test -count=1 -v -run TestCalibration ./internal/engine/...

# ── Performance regression gate ─────────────────────────────
# bench-baseline writes a fresh baseline benchmark snapshot. Run on a
# main-branch commit and commit the result.
# bench-gate runs the same benchmarks now and compares against the
# committed baseline; fails if any benchmark regressed >10%.
bench-baseline:
	go test -run '^$$' -bench 'BenchmarkRunPipeline|BenchmarkSignalDetection|BenchmarkBuildImportGraph|BenchmarkRiskScore|BenchmarkExtractTestCases' \
		-count=5 ./internal/engine ./internal/analysis ./internal/scoring ./internal/testcase \
		> benchmarks/baseline.txt
	@echo "Wrote benchmarks/baseline.txt"

# `memory-bench` runs the memory ceiling + leak-detection tests
# (TestMemoryCeiling_*, TestMemoryNoLeak_*). Skipped in the default
# `go test ./...` loop because they're slow (force GC + run analysis
# at scale) and surface ceiling regressions per the Track 9.10
# baseline. Set TERRAIN_MEMORY_BENCH=1 inline; this target does it
# for you.
memory-bench:
	@TERRAIN_MEMORY_BENCH=1 go test -v -count=1 -run 'TestMemory' ./internal/analysis/...

bench-gate:
	@tmp=$$(mktemp) ; \
	go test -run '^$$' -bench 'BenchmarkRunPipeline|BenchmarkSignalDetection|BenchmarkBuildImportGraph|BenchmarkRiskScore|BenchmarkExtractTestCases' \
		-count=5 ./internal/engine ./internal/analysis ./internal/scoring ./internal/testcase > $$tmp ; \
	go run ./cmd/terrain-bench-gate --base benchmarks/baseline.txt --head $$tmp --threshold 10 ; \
	rc=$$? ; \
	rm -f $$tmp ; \
	exit $$rc

release-verify:
	$(MAKE) go-release-verify
	$(MAKE) npm-release-verify
	$(MAKE) extension-verify

# ── Public Benchmark Matrix ──────────────────────────────────

# Download benchmark repos (shallow clone by default)
benchmark-fetch:
	./scripts/benchmarks/fetch_public_repos.sh

# Quick benchmark (smoke-tier repos only)
benchmark-smoke:
	./scripts/benchmarks/run_public_matrix.sh smoke
	python3 ./scripts/benchmarks/summarize_public_matrix.py

# Full benchmark matrix
benchmark-full:
	./scripts/benchmarks/run_public_matrix.sh full
	python3 ./scripts/benchmarks/summarize_public_matrix.py

# Stress benchmark (all repos including very large ones)
benchmark-stress:
	./scripts/benchmarks/run_public_matrix.sh stress
	python3 ./scripts/benchmarks/summarize_public_matrix.py

# Just regenerate the summary from existing artifacts
benchmark-summary:
	python3 ./scripts/benchmarks/summarize_public_matrix.py

# Compare current Go converters against the legacy JS runtime floor.
benchmark-convert:
	go run ./cmd/terrain-convert-bench
