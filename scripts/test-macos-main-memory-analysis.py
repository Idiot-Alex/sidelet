#!/usr/bin/env python3
"""Guard memory interpretation against shared residents, reservations and missing data."""
import importlib.util
from pathlib import Path
import unittest
import tempfile

spec=importlib.util.spec_from_file_location('main_memory',Path(__file__).with_name('analyze-macos-main-memory.py'))
module=importlib.util.module_from_spec(spec);spec.loader.exec_module(module)

SUMMARY='''
REGION TYPE VIRTUAL RESIDENT DIRTY SWAPPED VOLATILE NONVOL EMPTY REGION
JS JIT generated code 512.0M 1152K 400K 0K 0K 0K 0K 3
JS VM Gigacage 511.0M 256K 224K 0K 0K 0K 0K 3
JS VM Gigacage (reserved) 67.5G 0K 0K 0K 0K 0K 0K 3 reserved VM address space
MALLOC_SMALL 19.6M 7728K 4336K 0K 0K 0K 0K 412 see MALLOC ZONE table below
WebKit Malloc 96.0M 24.2M 16.2M 0K 0K 0K 0K 4
owned unmapped (graphics) 285.3M 285.3M 52.0M 0K 0K 0K 0K 63
MALLOC ZONE VIRTUAL RESIDENT DIRTY SWAPPED ALLOCATION BYTES
WebKit Malloc_0x100 992.1M 25.9M 17.9M 0K 71674 12.7M 5330K 30% 7
'''

class MemoryInterpretation(unittest.TestCase):
    def test_graphics_charge_is_dirty_not_shared_resident(self):
        categories=module.vmmap_categories(SUMMARY)
        self.assertEqual(categories['graphicsDirtyMiB'],52)
        self.assertEqual(categories['webkitMallocDirtyMiB'],16.2)

    def test_js_reservation_is_not_a_heap_allocation(self):
        categories=module.vmmap_categories(SUMMARY)
        self.assertEqual(categories['jsTaggedDirtyMiB'],624/1024)
        self.assertIn('not the complete',categories['jsTaggedScope'])

    def test_truncated_classification_is_not_silently_zero(self):
        with self.assertRaisesRegex(ValueError,'Missing graphics'):
            module.vmmap_categories(SUMMARY.replace('owned unmapped (graphics)','unknown mapping'))

    def test_unknown_size_unit_is_rejected(self):
        with self.assertRaises(ValueError):module.size_bytes('67.5T')

    def test_launch_order_does_not_identify_main(self):
        with tempfile.TemporaryDirectory() as temporary:
            root=Path(temporary);(root/'vmmap').mkdir()
            (root/'vmmap/first.txt').write_text('Process: WebContent [100]\n'+SUMMARY.replace('52.0M','624K'))
            (root/'vmmap/second.txt').write_text('Process: WebContent [101]\n'+SUMMARY)
            self.assertEqual(module.main_classification(root)[0],101)

    def test_ambiguous_main_is_rejected(self):
        with tempfile.TemporaryDirectory() as temporary:
            root=Path(temporary);(root/'vmmap').mkdir()
            for pid in (100,101):
                (root/f'vmmap/{pid}.txt').write_text(f'Process: WebContent [{pid}]\n'+SUMMARY)
            with self.assertRaisesRegex(ValueError,'Ambiguous'):
                module.main_classification(root)

if __name__=='__main__':unittest.main()
