import json
from pathlib import Path
import tempfile
import unittest
from evaluate import load_cases, percentile, word_error_rate

class EvaluationTests(unittest.TestCase):
    def test_percentile_handles_empty_and_tail(self):
        self.assertIsNone(percentile([], .95))
        self.assertEqual(percentile(list(range(1, 101)), .95), 95)
    def test_word_error_detects_missing_words(self):
        self.assertEqual(word_error_rate('полис продлить', 'полис продлить'), 0)
        self.assertEqual(word_error_rate('полис продлить', 'полис'), .5)
    def test_empty_dataset_fails_instead_of_perfect_score(self):
        with tempfile.TemporaryDirectory() as d:
            p = Path(d) / 'data.json'
            p.write_text(json.dumps({'cases': []}))
            with self.assertRaises(ValueError): load_cases(p)

if __name__ == '__main__': unittest.main()
