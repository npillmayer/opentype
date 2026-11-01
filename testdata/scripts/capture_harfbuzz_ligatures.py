#!/usr/bin/env python3
"""Capture/check SHAPE-019 oracles with HarfBuzz 15; --write replaces them."""
import argparse
import json
import shutil
from capture_harfbuzz_text import ROOT, capture

DEST = ROOT / "internal/hbcmp/testdata/text/ligatures.json"
CASES = [(ident, "ShaperTestLigatureMarks.ttf", "Latn", "en", "ltr", text, features)
         for ident, text, features in [
    ("ligature_intervening", "f\u0305i", []),
    ("ligature_trailing", "fi\u0305", []),
    ("ligature_many_marks", "f\u0305\u030di\u0305", []),
    ("ligature_nested_left", "f\u0305i\u0305l\u0305", []),
    ("ligature_nested_right", "f\u0305i\u0305l\u0305", ["liga=0"]),
    ("ligature_distinct_instances", "f\u0305i fi\u0305", []),
    ("ligature_input_glyph", "\ufb01\u0305", []),
    ("ligature_later_single", "f\u0305i", ["ss01=1"]),
    ("ligature_later_multiple", "f\u0305i", ["ss02=1"]),
]]
CASES.append(("ligature_hebrew_rtl", "ShaperTestLigatureMarks.ttf", "Hebr", "he", "rtl", "\u05d0\u05b0\u05d1", []))

if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--write", action="store_true")
    args = parser.parse_args()
    binary = shutil.which("hb-shape")
    if binary is None:
        parser.error("hb-shape 15.0.0 is required only for preparation")
    data = (json.dumps(capture(binary, CASES), indent=2, ensure_ascii=False)+"\n").encode()
    if args.write:
        DEST.write_bytes(data)
    elif not DEST.is_file() or DEST.read_bytes() != data:
        raise SystemExit("ligature capture differs; review before --write")
    print(f"{'Captured' if args.write else 'Verified'} {len(CASES)} ligature oracles")
