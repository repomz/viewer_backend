#!/usr/bin/env python3
"""Cell-addressed OCR of the department's printed monthly roster.

No language model assigns dates. Perspective/grid geometry determines addresses;
Tesseract reads bounded cells. Ambiguous ink is retained as a review flag.
Stdout is JSON only; photos and temporary OCR inputs never persist on disk.
"""
import calendar
from collections import Counter
import concurrent.futures
import ctypes
import ctypes.util
import csv
import io
import json
import os
import re
import subprocess
import sys
import threading

import cv2
import numpy as np

cv2.setNumThreads(1)
os.environ["OMP_THREAD_LIMIT"] = "1"
VALID = {"5", "6", "8", "16", "18", "24", "О", "Б"}
MONTHS = ["январ", "феврал", "март", "апрел", "ма", "июн", "июл", "август", "сентябр", "октябр", "ноябр", "декабр"]


class TesseractEngine:
    """Reuse loaded language data; each worker owns its own native handle."""
    def __init__(self, language):
        self.handle = None
        self.lib = ctypes.CDLL(ctypes.util.find_library("tesseract") or "libtesseract.so")
        ptr, integer, string = ctypes.c_void_p, ctypes.c_int, ctypes.c_char_p
        signatures = {
            "TessBaseAPICreate": ([], ptr),
            "TessBaseAPIInit3": ([ptr, string, string], integer),
            "TessBaseAPISetVariable": ([ptr, string, string], integer),
            "TessBaseAPISetPageSegMode": ([ptr, integer], None),
            "TessBaseAPISetImage": ([ptr, ptr, integer, integer, integer, integer], None),
            "TessBaseAPIGetTsvText": ([ptr, integer], ptr),
            "TessDeleteText": ([ptr], None),
            "TessBaseAPIClear": ([ptr], None),
            "TessBaseAPIClearAdaptiveClassifier": ([ptr], None),
            "TessBaseAPIDelete": ([ptr], None),
        }
        for name, (args, result) in signatures.items():
            function = getattr(self.lib, name)
            function.argtypes, function.restype = args, result
        self.handle = self.lib.TessBaseAPICreate()
        if not self.handle or self.lib.TessBaseAPIInit3(self.handle, None, language.encode()) != 0:
            raise RuntimeError("Не удалось загрузить язык распознавателя")

    def __del__(self):
        if self.handle:
            self.lib.TessBaseAPIDelete(self.handle)

    def read(self, image, psm, whitelist):
        pixels = np.ascontiguousarray(image, dtype=np.uint8)
        self.lib.TessBaseAPISetVariable(self.handle, b"tessedit_char_whitelist", (whitelist or "").encode())
        self.lib.TessBaseAPISetPageSegMode(self.handle, psm)
        self.lib.TessBaseAPIClearAdaptiveClassifier(self.handle)
        self.lib.TessBaseAPISetImage(self.handle, pixels.ctypes.data, pixels.shape[1], pixels.shape[0], 1, pixels.strides[0])
        text = self.lib.TessBaseAPIGetTsvText(self.handle, 0)
        try:
            if not text:
                raise RuntimeError("Ошибка распознавания ячейки")
            # Native TSV omits the header supplied by the command-line renderer.
            return "level\tpage_num\tblock_num\tpar_num\tline_num\tword_num\tleft\ttop\twidth\theight\tconf\ttext\n" + ctypes.string_at(text).decode()
        finally:
            if text:
                self.lib.TessDeleteText(text)
            self.lib.TessBaseAPIClear(self.handle)


_engines = threading.local()


def subprocess_ocr(image, language="eng", psm=7, whitelist=None):
    command = ["tesseract", "stdin", "stdout", "-l", language, "--psm", str(psm)]
    if whitelist:
        command += ["-c", "tessedit_char_whitelist=" + whitelist]
    command += ["-c", "tessedit_create_tsv=1"]
    result = subprocess.run(command, input=cv2.imencode(".png", image)[1].tobytes(),
                            stdout=subprocess.PIPE, stderr=subprocess.PIPE, timeout=8, check=True)
    return result.stdout.decode()


def ocr(image, language="eng", psm=7, whitelist=None):
    engines = getattr(_engines, "languages", None)
    if engines is None:
        engines = _engines.languages = {}
    if language not in engines:
        engines[language] = TesseractEngine(language)
    output = engines[language].read(image, psm, whitelist)
    words = [r for r in csv.DictReader(io.StringIO(output), delimiter="\t")
             if r.get("text", "").strip()]
    return " ".join(r["text"] for r in words), min((float(r["conf"]) for r in words), default=0)


def peaks(projection, cutoff, gap=3):
    indices = np.flatnonzero(projection > cutoff)
    return [int(np.mean(g)) for g in np.split(indices, np.flatnonzero(np.diff(indices) > gap) + 1) if len(g)]


def grid(source, days, turns=0):
    image = cv2.imdecode(np.frombuffer(source, np.uint8), cv2.IMREAD_COLOR)
    if image is None:
        raise ValueError("Фотография не читается")
    image = np.ascontiguousarray(np.rot90(image, turns))
    image = cv2.resize(image, None, fx=2000/image.shape[1], fy=2000/image.shape[1])
    gray = cv2.cvtColor(image, cv2.COLOR_BGR2GRAY)
    binary = cv2.adaptiveThreshold(gray, 255, cv2.ADAPTIVE_THRESH_GAUSSIAN_C, cv2.THRESH_BINARY_INV, 35, 15)
    contours, _ = cv2.findContours(cv2.morphologyEx(binary, cv2.MORPH_CLOSE, np.ones((3, 3), np.uint8)), cv2.RETR_LIST, cv2.CHAIN_APPROX_SIMPLE)
    quad = None
    for contour in sorted(contours, key=cv2.contourArea, reverse=True)[:20]:
        points = cv2.approxPolyDP(contour, cv2.arcLength(contour, True)*.015, True)
        if len(points) == 4 and cv2.contourArea(contour) > image.shape[0]*image.shape[1]*.35:
            quad = points.reshape(4, 2).astype(np.float32)
            break
    if quad is None:
        raise ValueError("Не найдены четыре границы таблицы. Снимите весь лист")
    sums, differences = quad.sum(axis=1), np.diff(quad, axis=1).ravel()
    ordered = np.array([quad[sums.argmin()], quad[differences.argmin()], quad[sums.argmax()], quad[differences.argmax()]], np.float32)
    warped = cv2.warpPerspective(gray, cv2.getPerspectiveTransform(ordered, np.float32([[0, 0], [2999, 0], [2999, 1949], [0, 1949]])), (3000, 1950))
    binary = cv2.adaptiveThreshold(warped, 255, cv2.ADAPTIVE_THRESH_GAUSSIAN_C, cv2.THRESH_BINARY_INV, 45, 15)
    horizontal = cv2.morphologyEx(binary, cv2.MORPH_OPEN, cv2.getStructuringElement(cv2.MORPH_RECT, (130, 1)))
    vertical = cv2.morphologyEx(binary, cv2.MORPH_OPEN, cv2.getStructuringElement(cv2.MORPH_RECT, (1, 130)))
    xs = peaks(np.count_nonzero(vertical, axis=0), 150, 5)
    ys = peaks(np.count_nonzero(horizontal, axis=1), 450, 5)
    # Near-duplicate projections are the two edges of a bent/thick ruled line.
    merged = []
    for y in ys:
        if merged and y-merged[-1] < 22:
            merged[-1] = (merged[-1]+y)//2
        else:
            merged.append(y)
    ys = merged
    if len(xs) not in (days+4, 35) or len(ys) < 15 or len(ys) > 70:
        raise ValueError("Не удалось однозначно разделить дни и строки таблицы")
    widths = np.diff(xs[3:])
    if widths.max() > widths.min()*1.65:
        raise ValueError("Повреждена сетка календарных дней")
    return warped, binary, horizontal, vertical, xs, ys


def read_cell(cell):
    # Crop to actual glyphs: PSM 7 otherwise interprets surrounding table space.
    count, labels, stats, _ = cv2.connectedComponentsWithStats(cell)
    mask = np.zeros_like(cell)
    for index in range(1, count):
        x, y, w, h, area = stats[index]
        if area >= 22 and h >= 10 and w < cell.shape[1]-2 and h < cell.shape[0]-1:
            mask[labels == index] = 255
    ys, xs = np.nonzero(mask)
    if len(xs) < 30:
        return {"value": "", "uncertain": bool(np.count_nonzero(cell) >= 45)}
    glyph = mask[ys.min():ys.max()+1, xs.min():xs.max()+1]
    crop = cv2.copyMakeBorder(255-glyph, 12, 12, 12, 12, cv2.BORDER_CONSTANT, value=255)
    crop = cv2.resize(crop, None, fx=3, fy=3)
    readings = []
    for mode in ((7, 8, 10) if glyph.shape[1] < glyph.shape[0]*1.2 else (7, 8)):
        text, confidence = ocr(crop, psm=mode, whitelist="0124568OB")
        value = text.replace(" ", "").replace("0", "О").replace("O", "О").replace("B", "Б")
        readings.append((value, confidence))
        if len(readings) == 2 and readings[0][0] == readings[1][0] and value in VALID and max(c for _, c in readings) >= 35:
            break  # A third vote cannot change an already confident majority.
    valid = [(v, c) for v, c in readings if v in VALID]
    votes = Counter(v for v, _ in valid)
    if valid and (len(votes) == 1 or votes.most_common(1)[0][1] >= 2):
        value = votes.most_common(1)[0][0]
        # Agreement alone does not certify noisy grey-column artefacts.
        return {"value": value, "uncertain": max(c for v, c in valid if v == value) < 35}
    return {"value": "", "uncertain": True}


def merge_row(rows, name, group, role, values):
    previous = next((r for r in rows if r["name"] == name and r["group"] == group and r["row"] == role), None)
    if previous:
        for old, new in zip(previous["cells"], values):
            if new["value"]:
                old["uncertain"] |= bool(old["value"] and old["value"] != new["value"])
                old["value"] = new["value"]
            old["uncertain"] |= new["uncertain"]
    else:
        rows.append({"name": name, "group": group, "row": role, "cells": values})


def nurse_rows(values):
    # Printed nurses can have separate or mixed day/night rows. Keep both
    # assignments on a date instead of overwriting six hours with eighteen.
    leave_role = "day" if any(c["value"] in {"5", "6", "8"} for c in values) else "duty"
    for role in ("day", "duty"):
        cells = []
        for cell in values:
            value = cell["value"]
            belongs = (value in {"5", "6", "8"}) if role == "day" else (value in {"16", "18", "24"})
            belongs |= value in {"О", "Б"} and role == leave_role
            cells.append({"value": value if belongs else "", "uncertain": cell["uncertain"] and (belongs or not value)})
        if any(c["value"] or c["uncertain"] for c in cells):
            yield role, cells


def recognize(source, month):
    year, number = map(int, month.split("-"))
    days = calendar.monthrange(year, number)[1]
    failure = "Не удалось определить ориентацию и сетку таблицы"
    for turns in range(4):
        try:
            gray, binary, horizontal, vertical, xs, ys = grid(source, days, turns)
        except ValueError:
            continue
        title, _ = ocr(cv2.resize(gray[ys[0]+5:ys[1]-4, xs[3]:-5], None, fx=2, fy=2), "rus", psm=11)
        if str(year) in title and MONTHS[number-1] in title.lower():
            break
        failure = "Месяц на фотографии не совпадает с выбранным или не читается"
    else:
        raise ValueError(failure)
    rows = []
    group = "surgeons"
    separator = False
    with concurrent.futures.ThreadPoolExecutor(max_workers=2) as pool:
        for row in range(2, len(ys)-1):
            name_image = gray[ys[row]+6:ys[row+1]-5, xs[1]+5:xs[2]-5]
            text, name_confidence = ocr(cv2.resize(name_image, None, fx=3, fy=3), "rus")
            match = re.search(r"([А-ЯЁ][а-яё]+)\s+([А-ЯЁ3])\s*[.,]?\s*([А-ЯЁ3])", text)
            if not match:
                role_text, _ = ocr(cv2.resize(gray[ys[row]+5:ys[row+1]-4, xs[2]+5:xs[3]-5], None, fx=3, fy=3), "rus")
                has_shift_label = bool(re.search(r"8\W*14|14\W*8|м\W*с|деж", role_text.lower()))
                if has_shift_label or (name_confidence >= 20 and len(re.findall(r"[А-Яа-яЁё]", text)) >= 8):
                    raise ValueError("Не удалось надёжно прочитать фамилию сотрудника. Сделайте более ровный снимок")
                if rows:
                    if group == "nurses":
                        break
                    separator = True
                continue
            if separator:
                group = "nurses"
            name = f"{match[1]} {match[2]}.{match[3]}.".replace("3", "З")
            if name.lower().startswith("терешин "):
                continue
            role = "duty"
            # Suslov has one duty-only row by department convention; this
            # month's printed form leaves his shift-label cell empty.
            if group == "surgeons" and not name.lower().startswith("суслов "):
                role_text, _ = ocr(cv2.resize(gray[ys[row]+5:ys[row+1]-4, xs[2]+5:xs[3]-5], None, fx=3, fy=3), psm=7, whitelist="012468-")
                role_text = role_text.replace(" ", "")
                if role_text in ("8-14", "814"):
                    role = "day"
                elif role_text not in ("14-8", "148"):
                    raise ValueError("Не удалось прочитать вид смены хирурга")
            cells = []
            for day in range(1, days+1):
                x1, x2 = xs[day+2:day+4]
                y1, y2 = ys[row:row+2]
                left = max(0, x1-11)+int(np.argmax(vertical[y1+7:y2-7, max(0, x1-11):x1+12].sum(axis=0)))
                right = x2 if day == days else x2-11+int(np.argmax(vertical[y1+7:y2-7, x2-11:min(gray.shape[1], x2+12)].sum(axis=0)))
                top = max(0, y1-12)+int(np.argmax(horizontal[max(0, y1-12):y1+12, left+5:right-5].sum(axis=1)))
                bottom = y2-12+int(np.argmax(horizontal[y2-12:min(gray.shape[0], y2+12), left+5:right-5].sum(axis=1)))
                cells.append(binary[top+5:bottom-4, left+5:right-4])
            values = list(pool.map(read_cell, cells))
            if group == "nurses":
                for nurse_role, cells in nurse_rows(values):
                    merge_row(rows, name, group, nurse_role, cells)
            else:
                merge_row(rows, name, group, role, values)
    if not any(r["group"] == "nurses" for r in rows):
        raise ValueError("Не найдена отдельная группа медсестёр")
    return {"month": month, "rows": rows}


if __name__ == "__main__":
    try:
        result = recognize(sys.stdin.buffer.read(12*1024*1024+1), sys.argv[1])
        print(json.dumps(result, ensure_ascii=False))
    except Exception as error:
        # No image bytes or recognized personal data in service error output.
        print(str(error) if isinstance(error, ValueError) else "Ошибка локального распознавания таблицы", file=sys.stderr)
        sys.exit(1)
