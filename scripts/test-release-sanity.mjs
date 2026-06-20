#!/usr/bin/env node

import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs/promises';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const rootDir = path.resolve(__dirname, '..');

async function readJSON(relPath) {
  return JSON.parse(await fs.readFile(path.join(rootDir, relPath), 'utf8'));
}

async function readText(relPath) {
  return fs.readFile(path.join(rootDir, relPath), 'utf8');
}

test('release package versions are aligned to 0.3.1', async () => {
  const rootPackage = await readJSON('package.json');
  const rootLock = await readJSON('package-lock.json');
  const extensionPackage = await readJSON('extension/vscode/package.json');
  const extensionLock = await readJSON('extension/vscode/package-lock.json');
  const legacyVSCodePackage = await readJSON('vscode/package.json');

  assert.equal(rootPackage.version, '0.3.1');
  assert.equal(rootLock.version, rootPackage.version);
  assert.equal(rootLock.packages[''].version, rootPackage.version);
  assert.equal(extensionPackage.version, rootPackage.version);
  assert.equal(extensionLock.version, rootPackage.version);
  assert.equal(extensionLock.packages[''].version, rootPackage.version);
  assert.equal(legacyVSCodePackage.version, rootPackage.version);
});

test('release workflow archives docs, signs checksums, and waits for smoke before npm', async () => {
  const workflow = await readText('.github/workflows/release.yml');

  assert.match(
    workflow,
    /cp "\$\{GITHUB_WORKSPACE\}\/README\.md" "\$stage\/README\.md"/
  );
  assert.match(
    workflow,
    /cp "\$\{GITHUB_WORKSPACE\}\/LICENSE" "\$stage\/LICENSE"/
  );
  assert.match(
    workflow,
    /7z a "\$\{DIST_ABS\}\/\$\{archive\}" "\$bin_name" README\.md LICENSE/
  );
  assert.match(
    workflow,
    /tar -czf "\$archive" -C "\$stage" "\$bin_name" README\.md LICENSE/
  );
  assert.match(
    workflow,
    /for f in \*\.tar\.gz \*\.zip \*\.cdx\.json \*\.spdx\.json checksums\.txt/
  );
  assert.match(
    workflow,
    /needs: \[verify, go-release-publish, release-smoke\]/
  );
});

test('goreleaser config no longer uses deprecated brews block', async () => {
  const config = await readText('.goreleaser.yaml');

  assert.doesNotMatch(config, /^brews:/m);
});

test('AI gate preserves structured JSON and documented exit codes', async () => {
  const main = await readText('cmd/terrain/main.go');
  const workflow = await readText('.github/workflows/terrain-ai.yml');

  assert.match(
    main,
    /case "run":[\s\S]*runAIRunWithTimeout[\s\S]*os\.Exit\(exitCodeForCLIError\(err\)\)/
  );
  assert.doesNotMatch(workflow, /\/tmp\/ai-run\.json 2>&1/);
  assert.match(
    workflow,
    /--json > \/tmp\/ai-run\.json 2> \/tmp\/ai-run\.stderr/
  );
  assert.match(workflow, /jq -e '\.decision\.action' \/tmp\/ai-run\.json/);
  assert.match(workflow, /reason<<TERRAIN_AI_REASON/);
});

test('0.3.0 public claims avoid known overstatements', async () => {
  const readme = await readText('README.md');
  const overview = await readText('docs/OVERVIEW.md');
  const limitations = await readText('docs/LIMITATIONS.md');
  const cliSpec = await readText('docs/cli-spec.md');
  const compatibility = await readText('docs/compatibility.md');
  const glossary = await readText('docs/glossary.md');
  const featureStatus = await readText('docs/release/feature-status.md');
  const securityData = await readText('SECURITY-DATA-HANDLING.md');
  const design = await readText('DESIGN.md');
  const product = await readText('docs/PRODUCT.md');
  const provider = await readText('internal/llmprovider/provider.go');
  const gauntletIntegration = await readText('docs/integrations/gauntlet.md');
  const docsReadme = await readText('docs/README.md');
  const legacyVSCodeReadme = await readText('vscode/README.md');
  const releaseVSCodeReadme = await readText('extension/vscode/README.md');

  assert.doesNotMatch(readme, /Go, Java, and Ruby/);
  assert.match(readme, /Ruby source is not analyzed in 0\.3\.0/);
  assert.match(readme, /Windows on amd64/);
  assert.doesNotMatch(readme, /0\.91\+ similarity/);
  assert.doesNotMatch(readme, /xfail markers older than 180 days/);

  assert.doesNotMatch(overview, /Marketplace-published/);
  assert.match(limitations, /not Marketplace-published in 0\.3\.0/);
  assert.doesNotMatch(limitations, /Marketplace-published but/);

  for (const doc of [overview, limitations]) {
    assert.doesNotMatch(doc, /renders findings in the IDE Problems pane/);
  }

  assert.match(
    cliSpec,
    /terrain portfolio --from <manifest>` multi-repo aggregation are stable in 0\.3\.0/
  );
  assert.match(
    featureStatus,
    /multi-repo aggregation via `terrain portfolio --from <manifest>` are stable in 0\.3\.0/
  );
  for (const flag of [
    '--promptfoo-results',
    '--deepeval-results',
    '--ragas-results',
    '--great-expectations-results',
  ]) {
    assert.match(
      cliSpec,
      new RegExp(flag),
      `docs/cli-spec.md must document ${flag}`
    );
  }
  assert.match(docsReadme, /integrations\/great-expectations\.md/);
  assert.match(docsReadme, /\[Release Notes\]\(release\/release-notes\.md\)/);
  for (const doc of [readme, compatibility, glossary]) {
    assert.match(doc, /Great Expectations/);
  }
  assert.match(legacyVSCodeReadme, /not the release extension/);
  assert.doesNotMatch(legacyVSCodeReadme, /Problems pane|Problems-pane/);
  assert.match(releaseVSCodeReadme, /pre-flight graph/);
  assert.doesNotMatch(releaseVSCodeReadme, /all 22\+ signal types/);
  assert.match(
    gauntletIntegration,
    /`terrain ai run` does not invoke Gauntlet in 0\.3\.0/
  );
  assert.doesNotMatch(
    gauntletIntegration,
    /invokes Gauntlet as the execution backend/
  );
  assert.doesNotMatch(
    overview,
    /GitHub Actions or GitLab CI workflow snippet provided/
  );
  assert.match(
    overview,
    /GitHub Actions template provided; other CI can invoke the CLI and consume JUnit/
  );
  assert.doesNotMatch(
    limitations,
    /Other CI platforms beyond GitHub Actions and GitLab CI/
  );
  for (const doc of [cliSpec, featureStatus]) {
    assert.doesNotMatch(doc, /portfolio --from <manifest>.*future work/);
    assert.doesNotMatch(doc, /aggregation is not wired in 0\.3\.0/);
    assert.doesNotMatch(doc, /portfolio --from <manifest>.*experimental/);
  }

  assert.match(
    provider,
    /tool calls not implemented for this provider in 0\.3\.0/
  );
  assert.doesNotMatch(
    provider,
    /tool calls not implemented for this provider at 0\.2\.0/
  );

  assert.match(securityData, /No LLM provider is contacted in 0\.3\.0/);
  assert.match(
    securityData,
    /npm binary matrix is macOS\/Linux amd64\+arm64 and Windows amd64/
  );
  assert.match(securityData, /No remote telemetry/);
  assert.match(securityData, /Optional local-only telemetry/);
  assert.match(overview, /No remote telemetry/);
  assert.match(product, /Optional local telemetry/);
  assert.doesNotMatch(
    limitations,
    /not yet implemented; future feature behind explicit config/
  );
  assert.match(
    product,
    /provider config is parsed for forward compatibility but no shipped command contacts an LLM provider/
  );
  assert.match(
    securityData,
    /source-content redaction is not active in 0\.3\.0/
  );
  assert.match(
    product,
    /`on_terrain_error: pass` field is parsed but inactive in 0\.3\.0/
  );
  assert.match(design, /source-content redaction is inactive in 0\.3\.0/);
  assert.match(design, /no shipped 0\.3\.0 command contacts an LLM provider/);
  for (const doc of [readme, overview, securityData, product, design]) {
    assert.doesNotMatch(doc, /Optional LLM tiers/);
    assert.doesNotMatch(doc, /Optional LLM-enhanced features layer on/);
    assert.doesNotMatch(doc, /Ollama default/);
    assert.doesNotMatch(
      doc,
      /terrain describe` calls the configured LLM provider/
    );
    assert.doesNotMatch(
      doc,
      /redact_source: true.*redacts source-code excerpts/
    );
    assert.doesNotMatch(
      doc,
      /stringent code-confidentiality requirements can set `redact_source: true`/
    );
    assert.doesNotMatch(
      doc,
      /can't tolerate this set `on_terrain_error: pass`/
    );
    assert.doesNotMatch(doc, /darwin\/linux\/windows.*amd64\/arm64/i);
  }

  const currentScopeDocs = [
    'docs/signals/manifest.json',
    'docs/examples/gate/ai-eval-ci/README.md',
    'docs/rules/_template.md',
    'docs/rules/coverage/no-integration-test.md',
    'docs/rules/coverage/no-tests.md',
    'docs/rules/data/leakage-suspected.md',
    'docs/rules/data/missing-train-test-split.md',
    'docs/rules/hygiene/eval-no-assertion.md',
    'docs/rules/hygiene/model-fixture-unpinned.md',
    'docs/rules/hygiene/secrets-in-prompt.md',
    'docs/rules/performance/missing-perf-test.md',
    'docs/rules/regression/eval-regression.md',
    'docs/rules/regression/snapshot-mismatch.md',
    'docs/rules/regression/test-failed.md',
    'docs/rules/reproducibility/missing-env-pinning.md',
    'docs/rules/reproducibility/no-seed.md',
    'docs/rules/reproducibility/version-floating.md',
    'docs/rules/security/insecure-deserialization.md',
    'docs/rules/security/pii-in-eval.md',
    'docs/rules/structural/uncovered-ai-surface.md',
    'internal/signals/manifest.go',
  ];
  const staleCurrentScopePatterns = [
    /Edge cases NOT handled at 0\.2\.0/,
    /0\.2\.0 scope:/,
    /0\.2\.0 implementation:/,
    /0\.2\.0 silence rule:/,
    /Patterns at 0\.2\.0/,
    /PII vocabulary at 0\.2\.0/,
    /0\.2\.0 deferred:/,
    /0\.2\.0 doesn't read lockfiles/,
    /responsibility in 0\.2\./,
    /calibrated in 0\.2 against/,
  ];
  for (const docPath of currentScopeDocs) {
    const doc = await readText(docPath);
    for (const pattern of staleCurrentScopePatterns) {
      assert.doesNotMatch(
        doc,
        pattern,
        `${docPath} contains stale current-scope wording`
      );
    }
  }
});
