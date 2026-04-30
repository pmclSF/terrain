# Terrain — signal-first test intelligence platform

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
DATE    ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS := -s -w -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)
GO_OWNED_PKGS := ./cmd/... ./internal/...

.PHONY: build test lint clean demo benchmark-fetch benchmark-smoke benchmark-full benchmark-stress benchmark-summary benchmark-convert install \
       test-golden test-determinism test-schema test-adversarial test-e2e test-cli test-bench golden-update pr-gate release-gate \
       sbom sbom-cyclonedx sbom-spdx release-dry-run go-release-verify js-release-verify extension-verify release-verify \
       docs-gen docs-verify

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
	@tmp=$$(mktemp -d) ; \
	go run ./cmd/terrain-docs-gen -out "$$tmp" ; \
	rc=0 ; \
	for f in docs/signals/manifest.json docs/severity-rubric.md ; do \
		if ! diff -u "$$f" "$$tmp/$$f" ; then \
			echo "::error::$$f is out of date. Run 'make docs-gen' and commit." ; \
			rc=1 ; \
		fi ; \
	done ; \
	rm -rf "$$tmp" ; \
	if [ $$rc -ne 0 ] ; then exit $$rc ; fi ; \
	echo "docs-verify: docs/signals/manifest.json + docs/severity-rubric.md are up to date."

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
