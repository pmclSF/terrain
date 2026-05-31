"""Reusable checkpoint helper for long-running validation pipelines.

Convention: any multi-stage Python pipeline under scripts/ that takes more
than ~10 minutes end-to-end MUST persist each stage's output to disk via this
helper. A Stage-N crash should never lose Stage-(N-1) work.

Usage — bulk-payload save/load:

    from _checkpoint import Checkpoint

    ck = Checkpoint(workdir='/tmp/my-pipeline-ckpts')

    if ck.has('stage1'):
        findings = ck.load('stage1')
    else:
        findings = expensive_clone_and_scan(...)  # 30 min on 500 repos
        ck.save('stage1', findings)

    # Stage 2 runs against findings; if it crashes here, Stage 1 work
    # is safe on disk and the next run resumes from ck.load('stage1').

Usage — streaming JSONL writer (for incremental row-by-row work):

    with ck.jsonl_writer('ratings') as w:
        for row in candidates:
            verdict = expensive_oracle_call(row)  # paid per call
            w.write({'id': row['id'], 'verdict': verdict})
            # crash here only loses the in-flight oracle call; everything
            # written so far is on disk.

The jsonl_writer appends across re-runs, so a re-launch continues where the
last write left off. Callers responsible for de-dup-on-resume: load the
existing file first and skip ids already processed.

Atomicity: save() writes to <stage>.json.tmp and renames into place, so a
crash mid-write doesn't corrupt a previously-good checkpoint.
"""

import json
import os
from contextlib import contextmanager
from pathlib import Path


class Checkpoint:
    """Disk-backed multi-stage checkpoint store for a single pipeline run.

    All stages of one pipeline share a single Checkpoint instance + workdir.
    Different pipelines should use different workdirs to avoid stage-name
    collisions.
    """

    def __init__(self, workdir):
        self.workdir = Path(workdir)
        self.workdir.mkdir(parents=True, exist_ok=True)

    # ── bulk save/load (JSON) ─────────────────────────────────────────

    def has(self, stage):
        """Return True if `stage` checkpoint exists (either .json or .jsonl)."""
        return self._path(stage, 'json').exists() or self._path(stage, 'jsonl').exists()

    def save(self, stage, payload):
        """Atomic JSON write of `payload` for `stage`. Overwrites any prior
        checkpoint at the same stage name."""
        final = self._path(stage, 'json')
        tmp = final.with_suffix('.json.tmp')
        with tmp.open('w') as f:
            json.dump(payload, f, indent=2, sort_keys=True)
        os.replace(tmp, final)

    def load(self, stage):
        """Load the `stage` checkpoint. For JSON checkpoints, returns the
        parsed payload. For JSONL checkpoints, returns a list of parsed rows.

        Raises FileNotFoundError if neither .json nor .jsonl exists for the
        stage; callers should guard with has() or catch."""
        jpath = self._path(stage, 'json')
        if jpath.exists():
            with jpath.open() as f:
                return json.load(f)
        lpath = self._path(stage, 'jsonl')
        if lpath.exists():
            rows = []
            with lpath.open() as f:
                for line in f:
                    line = line.strip()
                    if line:
                        rows.append(json.loads(line))
            return rows
        raise FileNotFoundError(f"checkpoint not found for stage {stage!r}")

    def clear(self, stage):
        """Remove the `stage` checkpoint if present. No-op if absent."""
        for ext in ('json', 'jsonl', 'json.tmp', 'jsonl.tmp'):
            p = self._path(stage, ext)
            if p.exists():
                p.unlink()

    def stages_present(self):
        """Return the sorted list of stage names with a checkpoint on disk."""
        out = set()
        for p in self.workdir.iterdir():
            name = p.name
            for ext in ('.json', '.jsonl'):
                if name.endswith(ext):
                    out.add(name[: -len(ext)])
        return sorted(out)

    # ── streaming append writer (JSONL) ───────────────────────────────

    @contextmanager
    def jsonl_writer(self, stage):
        """Open an append-mode JSONL writer for `stage`. Yields an object
        with .write(obj) that serializes one JSON object per line and flushes
        immediately, so a crash mid-loop preserves everything written so far.

        On re-entry (subsequent runs), opens the same file in append mode —
        previously-written rows survive.

        Example:
            with ck.jsonl_writer('ratings') as w:
                for row in candidates:
                    w.write({'id': row.id, 'verdict': oracle(row)})
        """
        path = self._path(stage, 'jsonl')
        fh = path.open('a')
        try:
            yield _LineWriter(fh)
        finally:
            fh.flush()
            fh.close()

    # ── internals ─────────────────────────────────────────────────────

    def _path(self, stage, ext):
        return self.workdir / f"{stage}.{ext}"


class _LineWriter:
    """Newline-delimited JSON writer that flushes after every record."""

    def __init__(self, fh):
        self._fh = fh

    def write(self, obj):
        self._fh.write(json.dumps(obj, sort_keys=True) + "\n")
        self._fh.flush()
        try:
            os.fsync(self._fh.fileno())
        except (OSError, AttributeError):
            # Some platforms / filesystems don't support fsync; flush is
            # still useful for buffered-write protection.
            pass
