"""Tests for scripts/_checkpoint.py.

Run with: python3 scripts/_checkpoint_test.py
"""

import sys
import tempfile
import unittest
from pathlib import Path

sys.path.insert(0, str(Path(__file__).parent))

from _checkpoint import Checkpoint  # noqa: E402


class CheckpointTest(unittest.TestCase):
    def test_save_load_roundtrip(self):
        with tempfile.TemporaryDirectory() as tmp:
            ck = Checkpoint(workdir=tmp)
            self.assertFalse(ck.has('stage1'))
            payload = {'findings': [1, 2, 3], 'meta': {'n': 3}}
            ck.save('stage1', payload)
            self.assertTrue(ck.has('stage1'))
            self.assertEqual(ck.load('stage1'), payload)

    def test_load_missing_raises(self):
        with tempfile.TemporaryDirectory() as tmp:
            ck = Checkpoint(workdir=tmp)
            with self.assertRaises(FileNotFoundError):
                ck.load('absent')

    def test_jsonl_writer_streams(self):
        with tempfile.TemporaryDirectory() as tmp:
            ck = Checkpoint(workdir=tmp)
            with ck.jsonl_writer('ratings') as w:
                w.write({'id': 1, 'verdict': 'TP'})
                w.write({'id': 2, 'verdict': 'FP'})
            self.assertTrue(ck.has('ratings'))
            loaded = ck.load('ratings')
            self.assertEqual(len(loaded), 2)
            self.assertEqual(loaded[0]['verdict'], 'TP')

    def test_jsonl_append_across_runs(self):
        with tempfile.TemporaryDirectory() as tmp:
            ck = Checkpoint(workdir=tmp)
            with ck.jsonl_writer('ratings') as w:
                w.write({'id': 1})
            with ck.jsonl_writer('ratings') as w:
                w.write({'id': 2})
            self.assertEqual(len(ck.load('ratings')), 2)

    def test_atomic_save_uses_temp_file(self):
        with tempfile.TemporaryDirectory() as tmp:
            ck = Checkpoint(workdir=tmp)
            ck.save('stage1', {'x': 1})
            self.assertFalse((Path(tmp) / 'stage1.json.tmp').exists())
            self.assertTrue((Path(tmp) / 'stage1.json').exists())

    def test_stages_present(self):
        with tempfile.TemporaryDirectory() as tmp:
            ck = Checkpoint(workdir=tmp)
            self.assertEqual(ck.stages_present(), [])
            ck.save('stage1', {})
            ck.save('stage2', {})
            with ck.jsonl_writer('stage3') as w:
                w.write({})
            self.assertEqual(ck.stages_present(), ['stage1', 'stage2', 'stage3'])

    def test_clear(self):
        with tempfile.TemporaryDirectory() as tmp:
            ck = Checkpoint(workdir=tmp)
            ck.save('stage1', {})
            self.assertTrue(ck.has('stage1'))
            ck.clear('stage1')
            self.assertFalse(ck.has('stage1'))


if __name__ == '__main__':
    unittest.main()
