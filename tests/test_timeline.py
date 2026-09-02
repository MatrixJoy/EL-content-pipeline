import sys
import unittest

sys.path.insert(0, "alignment")
from timeline import build_timeline


class TimelineTests(unittest.TestCase):
    def test_maps_sentences_and_marks_unspoken_text(self):
        paragraphs = [{"index": 0, "text": "Hello world. This sentence was omitted. We learn English!"}]
        words = [
            {"word": "Hello", "start": 1.0, "end": 1.3},
            {"word": "world", "start": 1.4, "end": 1.8},
            {"word": "We", "start": 4.0, "end": 4.1},
            {"word": "learn", "start": 4.2, "end": 4.5},
            {"word": "English", "start": 4.6, "end": 5.0},
        ]
        result = build_timeline(paragraphs, words, 6000)
        self.assertEqual(3, len(result))
        self.assertEqual((1000, 1800), (result[0]["start_ms"], result[0]["end_ms"]))
        self.assertEqual((None, None), (result[1]["start_ms"], result[1]["end_ms"]))
        self.assertEqual((4000, 5000), (result[2]["start_ms"], result[2]["end_ms"]))
        self.assertEqual(0, result[1]["confidence"])
        self.assertFalse(result[1]["spoken"])


if __name__ == "__main__":
    unittest.main()
