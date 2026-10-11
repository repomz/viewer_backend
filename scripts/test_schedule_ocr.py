"""Run with python -m unittest discover -s scripts -p 'test_schedule_ocr.py'.

SCHEDULE_TEST_PHOTO enables the October reference-photo regression, including
every numeric shift and empty day, not merely the number of detected names.
The photograph itself is deliberately not committed.
"""
import os
from pathlib import Path
import unittest

import numpy as np
from schedule_ocr import read_cell, recognize


class RosterOCRTest(unittest.TestCase):
    def test_blank(self):
        self.assertEqual(read_cell(np.zeros((40, 70), np.uint8)), {"value": "", "uncertain": False})

    @unittest.skipUnless(os.getenv("SCHEDULE_TEST_PHOTO"), "reference photo not supplied")
    def test_reference_calendar_cells(self):
        result = recognize(Path(os.environ["SCHEDULE_TEST_PHOTO"]).read_bytes(), "2026-10")
        self.assertEqual(len(result["rows"]), 17)
        days = [1,2,5,6,7,8,9,12,13,14,15,16,19,20,21,22,23,26,27,28,29,30]
        expected = {
            ("Киргизов", "day"): dict.fromkeys(days, "6"),
            ("Идрисов", "day"): dict.fromkeys(days[2:], "6"),
            ("Старков", "day"): dict.fromkeys([d for d in days if d != 16], "6"),
            ("Шпилевой", "day"): dict.fromkeys(days, "6"),
            ("Киргизов", "duty"): dict.fromkeys([1,6,8,13,15,20,22,27], "18"),
            ("Идрисов", "duty"): {**dict.fromkeys([9,16,19,28,30], "18"), 3:"24",24:"24"},
            ("Старков", "duty"): {4:"24",7:"18",11:"24",14:"18",25:"24",29:"18"},
            ("Шпилевой", "duty"): {2:"18",10:"24",18:"24",23:"18",26:"18",31:"24"},
            ("Суслов", "duty"): {5:"18",12:"18",17:"24",21:"18"},
            ("Воронова", "duty"): {4:"24",11:"24",16:"16",28:"16"},
            ("Ельцова", "duty"): {2:"16",7:"16",10:"24",17:"24",**dict.fromkeys(range(19,32),"О")},
            ("Ильина", "duty"): {1:"16",5:"16",15:"16",20:"16",24:"24",27:"16"},
            ("Кайрова", "duty"): {6:"16",14:"16",25:"24",31:"24"},
            ("Протасова", "duty"): dict.fromkeys([8,13,19,22,26,29], "16"),
            ("Суханова", "duty"): {12:"16",18:"24",21:"16",30:"16"},
            ("Шишкина", "duty"): {23:"16",**dict.fromkeys(range(5,19),"О")},
            ("Штейнле", "duty"): {3:"24",9:"16",**dict.fromkeys(range(12,32),"О")},
        }
        certain = 0
        for row in result["rows"]:
            truth = expected[(row["name"].split()[0], row["row"])]
            self.assertEqual(len(row["cells"]),31)
            for day, cell in enumerate(row["cells"],1):
                if not cell["uncertain"]:
                    certain += 1
                    self.assertEqual(cell["value"], truth.get(day,""), (row["name"],row["row"],day))
        self.assertGreater(certain, 470, "too many cells require manual review")


if __name__ == "__main__":
    unittest.main()
