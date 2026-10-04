#!/usr/bin/env python3
import importlib.util
from pathlib import Path
import unittest

spec = importlib.util.spec_from_file_location("heap_analysis", Path(__file__).with_name("analyze-macos-heap.py"))
heap = importlib.util.module_from_spec(spec)
spec.loader.exec_module(heap)


class HeapAnalysisTest(unittest.TestCase):
    def test_listener_class_change_and_removed_class(self):
        before = heap.parse_heap("""All zones: 20 nodes (4000 bytes)
  10   800  80.0   WebCore::JSEventListener   C++   WebCore
   2    64  32.0   NSMutableArray   ObjC   CoreFoundation
""")
        after = heap.parse_heap("""All zones: 18 nodes (3900 bytes)
  11   880  80.0   WebCore::JSEventListener   C++   WebCore
""")
        result = heap.compare(before, after)
        self.assertEqual(result['deltaBytes'], -100)
        self.assertEqual(result['classes'][0]['deltaCount'], 1)
        self.assertEqual(result['classes'][1]['deltaCount'], -2)

    def test_errors_are_not_silently_treated_as_empty_heaps(self):
        for text in ["error: permission denied", "All zones: 20 nodes (4000 bytes)"]:
            with self.assertRaises(ValueError):
                heap.parse_heap(text)

    def test_same_class_with_multiple_layouts_is_summed(self):
        result = heap.parse_heap("All zones: 3 nodes (48 bytes)\n 1 16 16.0 NSArray   ObjC   CoreFoundation\n 2 32 16.0 NSArray   ObjC   CoreFoundation")
        self.assertEqual(result['classes'][('NSArray', 'ObjC', 'CoreFoundation')]['count'], 3)
        self.assertEqual(result['classes'][('NSArray', 'ObjC', 'CoreFoundation')]['bytes'], 48)

    def test_same_name_from_different_binaries_stays_separate(self):
        result = heap.parse_heap("All zones: 3 nodes (48 bytes)\n 1 16 16.0 Provider   C++   WebCore\n 2 32 16.0 Provider   C++   JavaScriptCore")
        self.assertEqual(len(result['classes']), 2)


if __name__ == '__main__':
    unittest.main()
