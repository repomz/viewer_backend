"""Run with python -m unittest discover -s scripts -p 'test_schedule_ocr.py'.

SCHEDULE_TEST_PHOTO enables the October reference-photo regression, including
every numeric shift and empty day, not merely the number of detected names.
The photograph itself is deliberately not committed.
"""
import os
from pathlib import Path
import unittest
import concurrent.futures
import csv
import io
import cv2

import numpy as np
from schedule_ocr import read_cell, recognize, ocr, subprocess_ocr, grid


class RosterOCRTest(unittest.TestCase):
    def test_reused_engine_matches_command_line(self):
        def compare(value):
            image = np.full((90, 190), 255, np.uint8)
            cv2.putText(image, value, (20, 65), cv2.FONT_HERSHEY_SIMPLEX, 2, 0, 3)
            for mode in (7, 8, 10):
                text, confidence = ocr(image, psm=mode, whitelist="012468OB")
                words = [r for r in csv.DictReader(io.StringIO(subprocess_ocr(image, psm=mode, whitelist="012468OB")), delimiter="\t") if r.get("text", "").strip()]
                self.assertEqual(text, " ".join(r["text"] for r in words))
                self.assertAlmostEqual(confidence, min((float(r["conf"]) for r in words), default=0), places=3)
        with concurrent.futures.ThreadPoolExecutor(max_workers=2) as pool:
            list(pool.map(compare, ["6", "16", "18", "24", "O", "6"]))

    def test_november_accepts_31_column_template_under_perspective(self):
        image = np.full((1500, 2400, 3), 255, np.uint8)
        xs = [50, 100, 460, 570] + list(range(625, 2276, 55))
        for x in xs:
            cv2.line(image, (x, 50), (x, 1400), (0, 0, 0), 3)
        for y in range(50, 1401, 50):
            cv2.line(image, (50, y), (2275, y), (0, 0, 0), 3)
        transform = cv2.getPerspectiveTransform(np.float32([[0, 0], [2399, 0], [2399, 1499], [0, 1499]]), np.float32([[35, 60], [2350, 10], [2380, 1460], [10, 1490]]))
        image = cv2.warpPerspective(image, transform, (2400, 1500), borderValue=(255, 255, 255))
        result = grid(cv2.imencode(".png", image)[1].tobytes(), 30)
        self.assertEqual(len(result[4]), 35)

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
