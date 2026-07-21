# terrain/security/pii-in-eval — PII in Eval Dataset

> Auto-generated stub. Edit anything below the marker; the generator preserves it.

**Type:** `piiInEval`  
**Domain:** ai  
**Default severity:** critical  
**Lifecycle status:** experimental  
**Gating tier:** gate

## Summary

An eval-directory file contains PII-shaped values (emails, phone numbers, SSNs, credit card numbers, IPv4 addresses). Eval datasets that retain production PII expose customer data to anyone with repo access.

## Remediation

Replace PII in the eval dataset with synthetic equivalents (Faker, Mimesis, mockaroo) or apply a redaction pass before committing.

## Evidence sources

- `structural-pattern`

## Confidence range

Confidence interval: 0.75–0.95.

<!-- docs-gen: end stub. Hand-authored content below this line is preserved across regenerations. -->

## 1. Summary

An eval-directory file contains PII-shaped values — emails, phone numbers, SSNs, credit card numbers, or IPv4 addresses. Eval datasets that retain production PII expose customer data to anyone with repo access.

## 2. Severity & status

Experimental — off by default; enable in `terrain.yaml`.

- **Default severity:** critical
- **Configurable via `terrain.yaml`:** yes — see [configuration.md](../../configuration.md)

## 3. What this catches

- A `.csv` under `evals/` whose rows include email-, SSN-, or phone-shaped values copied from production
- A `.jsonl` eval fixture whose prompt asks the model to send a receipt to a real user address
- A `.yaml` eval scenario whose expected output replays a production support ticket containing PII
- A Python docstring example in `evals/regression.py` that embeds a phone number

## 4. Why this matters

Eval datasets are committed to the repository, which means they're visible to every engineer with read access — typically more people than have production data access. The same compliance perimeter that protects production data (HIPAA, GDPR, PCI) doesn't follow data into an `evals/` directory by default. Production PII in eval files is one of the highest-impact accidental disclosures Terrain can prevent at the source-control gate.

The rule fires on a structural fact — PII-shaped values in eval-directory files — rather than runtime data flow. That structural check is sufficient for the gate because the question "is there PII in this file at HEAD" is exactly the gate-relevant question. The mitigation is either redaction or synthetic data; both produce eval files that don't trip the rule.

## 5. Detection mechanism

- **Approach:** path filter (eval directories) + content scan with PII regex vocabulary.
- **Paths considered:** `/eval/`, `/evals/`, `/evaluations/`, `/__evals__/`.
- **File types scanned:** `.yaml`, `.yml`, `.json`, `.jsonl`, `.csv`, `.tsv`, `.txt`, `.py`, `.md`. Binary / model artifact extensions skipped.
- **PII vocabulary:**
  - Email — `local@domain.tld`
  - US SSN — 3-2-4 digit groups with leading digit ≠ 9
  - US phone — NPA-NXX-XXXX with optional `+1` and any separator
  - IPv4 — dotted-quad
  - Credit card — 13-19 digit run with optional separators, leading digit 3/4/5/6
- **Confidence ladder:** single PII kind = 0.75; two kinds = 0.88; three or more = 0.95. Multi-kind matches are harder to explain as false positives.
- **Edge cases not handled:** richer named-entity detection (names, addresses, custom entity types) beyond the regex vocabulary above.

## 6. Worked example

```
critical[terrain/security/pii-in-eval]: eval-directory file contains PII-shaped values (email, phone-us, ssn)
  --> evals/leak.txt
   = help: Replace PII in the eval dataset with synthetic equivalents (Faker / Mimesis / mockaroo) or apply a redaction pass before committing.
   = docs: https://github.com/pmclSF/terrain/blob/main/docs/rules/security/pii-in-eval
```

**Before:**

```
# evals/customer_support.csv
ticket_id,subject,body
1234,Account help,"<production message containing email, phone, and SSN-shaped values>"
```

**After:**

```
# evals/customer_support.csv
ticket_id,subject,body
1234,Account help,"Synthetic customer fixture with email, phone, and SSN fields redacted or generated from a faker library"
```

## 7. Configuration

```yaml
rules:
  security/pii-in-eval: high   # downgrade if your team accepts synthetic-PII-shaped data
ignore:
  rules:
    security/pii-in-eval:
      - "evals/synthetic/**"
```

## 8. False-positive characterization

- **Synthetic data that happens to match a PII regex** (for example, famously fictitious phone-number values can still match the US phone pattern). Mitigation: ignore via path, or downgrade to high.
- **Email-shaped strings in code samples** (e.g., `email_regex = r"[A-Za-z0-9._%+\-]+@..."`). The rule scans line-by-line and doesn't distinguish a regex literal from data. Mitigation: ignore the file, or move the example out of `/evals/`.
- **Test fixtures intentionally using `example.com` / `noreply@example.com`** — `example.com` is RFC-2606 reserved for examples but the regex matches. Mitigation: same.

Adopters report false positives via the GitHub issue tracker with the originating snippet.

## 9. Reproducibility

```bash
terrain test --selector security/pii-in-eval
```

## 10. Stability commitment

Rule ID, severity, and the current PII vocabulary (email / SSN / phone-us / IPv4 / credit-card) are stable. Adding new entity types is additive and documented in CHANGELOG; removing types from the default set is deprecation-cycled.

## 11. Related rules

- `terrain/security/insecure-deserialization` — sister security rule for unsafe load patterns
- `terrain/hygiene/secrets-in-prompt` — sister rule for credentials embedded in prompts
- `terrain/data/leakage-suspected` — adjacent concern: train/test contamination at runtime
