#!/usr/bin/env python3
"""Cell-addressed OCR of the department's printed monthly roster.

No language model assigns dates. Perspective/grid geometry determines addresses;
Tesseract reads bounded cells. Ambiguous ink is retained as a review flag.
Stdout is JSON only; photos and temporary OCR inputs never persist on disk.
"""
import calendar
from collections import Counter
import concurrent.futures
import csv
import io
import json
import os
import re
import subprocess
import sys

import cv2
import numpy as np

cv2.setNumThreads(1)
os.environ["OMP_THREAD_LIMIT"] = "1"
VALID = {"6", "8", "16", "18", "24", "О", "Б"}
MONTHS = ["январ", "феврал", "март", "апрел", "ма", "июн", "июл", "август", "сентябр", "октябр", "ноябр", "декабр"]


def ocr(image, language="eng", psm=7, whitelist=None):
    command = ["tesseract", "stdin", "stdout", "-l", language, "--psm", str(psm)]
    if whitelist:
        command += ["-c", "tessedit_char_whitelist=" + whitelist]
    command += ["-c", "tessedit_create_tsv=1"]
    result = subprocess.run(command, input=cv2.imencode(".png", image)[1].tobytes(),
                            stdout=subprocess.PIPE, stderr=subprocess.PIPE, timeout=8, check=True)
    words = [r for r in csv.DictReader(io.StringIO(result.stdout.decode()), delimiter="\t")
             if r.get("text", "").strip()]
    return " ".join(r["text"] for r in words), min((float(r["conf"]) for r in words), default=0)


def peaks(projection, cutoff, gap=3):
    indices = np.flatnonzero(projection > cutoff)
    return [int(np.mean(g)) for g in np.split(indices, np.flatnonzero(np.diff(indices) > gap) + 1) if len(g)]


def grid(source, days):
    image = cv2.imdecode(np.frombuffer(source, np.uint8), cv2.IMREAD_COLOR)
    if image is None:
        raise ValueError("Фотография не читается")
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
    if len(xs) != days+4 or len(ys) < 15 or len(ys) > 70:
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
        text, confidence = ocr(crop, psm=mode, whitelist="012468OB")
        value = text.replace(" ", "").replace("0", "О").replace("O", "О").replace("B", "Б")
        readings.append((value, confidence))
    valid = [(v, c) for v, c in readings if v in VALID]
    votes = Counter(v for v, _ in valid)
    if valid and (len(votes) == 1 or votes.most_common(1)[0][1] >= 2):
        value = votes.most_common(1)[0][0]
        # Agreement alone does not certify noisy grey-column artefacts.
        return {"value": value, "uncertain": max(c for v, c in valid if v == value) < 35}
    return {"value": "", "uncertain": True}


def recognize(source, month):
    year, number = map(int, month.split("-"))
    days = calendar.monthrange(year, number)[1]
    gray, binary, horizontal, vertical, xs, ys = grid(source, days)
    title, _ = ocr(cv2.resize(gray[ys[0]+5:ys[1]-4, xs[3]:-5], None, fx=2, fy=2), "rus", psm=11)
    if str(year) not in title or MONTHS[number-1] not in title.lower():
        raise ValueError("Месяц на фотографии не совпадает с выбранным или не читается")
    rows = []
    group = "surgeons"
    separator = False
    with concurrent.futures.ThreadPoolExecutor(max_workers=2) as pool:
        for row in range(2, len(ys)-1):
            name_image = gray[ys[row]+6:ys[row+1]-5, xs[1]+5:xs[2]-5]
            text, _ = ocr(cv2.resize(name_image, None, fx=3, fy=3), "rus")
            match = re.search(r"([А-ЯЁ][а-яё]+)\s+([А-ЯЁ3])\s*[.,]?\s*([А-ЯЁ3])", text)
            if not match:
                if len(re.findall(r"[А-Яа-яЁё]", text)) >= 8:
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
            if group == "surgeons":
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
            previous = next((r for r in rows if r["name"] == name and r["group"] == group and r["row"] == role), None)
            if previous:
                for old, new in zip(previous["cells"], values):
                    if new["value"]:
                        old["uncertain"] |= bool(old["value"] and old["value"] != new["value"])
                        old["value"] = new["value"]
                    old["uncertain"] |= new["uncertain"]
            else:
                rows.append({"name": name, "group": group, "row": role, "cells": values})
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
