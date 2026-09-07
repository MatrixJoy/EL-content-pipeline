import sys
import unittest

sys.path.insert(0, "alignment")
from retry_state import FailureRetry
from timeline import build_timeline, timeline_stats


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

    def test_timeline_stats_does_not_count_incidental_word_matches_as_spoken_content(self):
        paragraphs = [{"index": 0, "text": "A complete sentence needs several matching words."}]
        words = [{"word": "sentence", "start": 1.0, "end": 1.2}]
        result = build_timeline(paragraphs, words, 2000)
        coverage, spoken_count = timeline_stats(result)
        self.assertGreater(coverage, 0)
        self.assertEqual(0, spoken_count)


class FailureRetryTests(unittest.TestCase):
    def test_failed_item_becomes_eligible_after_cooldown(self):
        now = [100.0]
        retry = FailureRetry(60, clock=lambda: now[0])
        self.assertTrue(retry.ready("lesson-1"))
        self.assertEqual(160.0, retry.failed("lesson-1"))
        self.assertFalse(retry.ready("lesson-1"))
        now[0] = 160.0
        self.assertTrue(retry.ready("lesson-1"))


if __name__ == "__main__":
    unittest.main()
