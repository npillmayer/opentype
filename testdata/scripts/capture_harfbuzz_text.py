#!/usr/bin/env python3
"""Capture real-text baselines with HarfBuzz, never with the repository shaper.

Requires hb-shape 15.0.0 for capture/check only. Default is read-only comparison;
use --write to replace the snapshot after reviewing any intended changes.
"""

import argparse
import hashlib
import json
import os
from pathlib import Path
import shutil
import subprocess

ROOT = Path(__file__).resolve().parents[2]
DEST = ROOT / "internal/hbcmp/testdata/text/baselines.json"
VERSION = "hb-shape (HarfBuzz) 15.0.0"
CASES = [
    ("latin_office_liga_on", "GentiumPlus-R.ttf", "Latn", "en", "ltr", "office", ["liga=1"]),
    ("latin_office_liga_off", "GentiumPlus-R.ttf", "Latn", "en", "ltr", "office", ["liga=0"]),
    ("latin_avatar_kern_on", "GentiumPlus-R.ttf", "Latn", "en", "ltr", "AVATAR", ["kern=1"]),
    ("latin_avatar_kern_off", "GentiumPlus-R.ttf", "Latn", "en", "ltr", "AVATAR", ["kern=0"]),
    ("latin_cafe_nfc", "GentiumPlus-R.ttf", "Latn", "fr", "ltr", "caf\u00e9", []),
    ("latin_cafe_nfd", "GentiumPlus-R.ttf", "Latn", "fr", "ltr", "cafe\u0301", []),
    ("hebrew_shin", "NotoSansHebrew-Regular.ttf", "Hebr", "he", "rtl", "\u05e9", []),
    ("hebrew_shin_pointed", "NotoSansHebrew-Regular.ttf", "Hebr", "he", "rtl", "\u05e9\u05b8\u05c1", []),
    ("hebrew_vav_holam", "NotoSansHebrew-Regular.ttf", "Hebr", "he", "rtl", "\u05d5\u05b9", []),
    ("hebrew_shalom", "NotoSansHebrew-Regular.ttf", "Hebr", "he", "rtl", "\u05e9\u05dc\u05d5\u05dd", []),
    ("hebrew_shalom_pointed", "NotoSansHebrew-Regular.ttf", "Hebr", "he", "rtl", "\u05e9\u05b8\u05c1\u05dc\u05d5\u05b9\u05dd", []),
]


def sha(data):
    return hashlib.sha256(data).hexdigest()


def capture(binary):
    env = {**os.environ, "LC_ALL": "C"}
    version = subprocess.check_output([binary, "--version"], text=True, env=env).strip()
    if version != VERSION:
        raise ValueError(f"expected {VERSION!r}, got {version!r}")
    cases = []
    for ident, font, script, language, direction, text, features in CASES:
        font_path = "testdata/fonts/" + font
        args = [font_path, "--unicodes=" + ",".join(f"U+{ord(c):04X}" for c in text),
                "--shapers=ot", "--font-funcs=ot", "--font-size=upem",
                "--script=" + script, "--language=" + language, "--direction=" + direction,
                "--cluster-level=0", "--single-par", "--no-glyph-names", "--output-format=json"]
        if features:
            args.append("--features=" + ",".join(features))
        output = json.loads(subprocess.check_output([binary, *args], cwd=ROOT, env=env, text=True))
        if not output or any(set(g) != {"g", "cl", "dx", "dy", "ax", "ay"} or
                             any(type(v) is not int for v in g.values()) or g["g"] == 0
                             for g in output):
            raise ValueError(f"{ident}: incomplete output or missing glyph")
        cases.append({"id": ident, "text": text, "font_path": font_path,
                      "font_sha256": sha((ROOT / font_path).read_bytes()),
                      "command": ["hb-shape", *args],
                      "fixture": {"schema_version": 1,
                                  "context": {"font": font, "script": script,
                                              "language": language, "dir": direction,
                                              "features": features},
                                  "input": [ord(c) for c in text], "output": output}})
    return {"schema_version": 1, "capture": {
        "version": version, "binary_sha256": sha(Path(binary).resolve().read_bytes()),
        "locale": "C", "cluster_indices": "unicode_scalars", "cluster_level": 0,
        "units": "font_units", "normalization": "HarfBuzz default; input unmodified"},
        "cases": cases}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--hb-shape", default="hb-shape")
    parser.add_argument("--write", action="store_true")
    args = parser.parse_args()
    binary = shutil.which(args.hb_shape)
    if binary is None:
        parser.error("hb-shape is required for capture/check, not for Go tests")
    data = (json.dumps(capture(binary), indent=2, ensure_ascii=False) + "\n").encode()
    if args.write:
        DEST.parent.mkdir(parents=True, exist_ok=True)
        DEST.write_bytes(data)
        print(f"Captured {len(CASES)} cases in {DEST.relative_to(ROOT)}")
    elif not DEST.is_file() or DEST.read_bytes() != data:
        raise SystemExit("capture differs (including tool/font provenance); review before --write")
    else:
        print(f"Verified {len(CASES)} independent captures")


if __name__ == "__main__":
    main()
