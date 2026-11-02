#!/usr/bin/env python3
"""Capture/check focused SHAPE-023 Hebrew ordering oracles with HarfBuzz 15."""
import argparse
import json
import shutil
from capture_harfbuzz_text import ROOT, capture

DEST = ROOT / "internal/hbcmp/testdata/text/hebrew_order.json"
OFF = ["mark=0", "mkmk=0"]
CASES = [(ident, "NotoSansHebrew-Regular.ttf", "Hebr", "he", "rtl", text, features)
         for ident, text, features in [
    ("hebrew_order_sin", "\u05e9\u05b8\u05c2", []),
    ("hebrew_order_dagesh", "\u05e9\u05b8\u05bc\u05c1", []),
    ("hebrew_order_equal_class", "\u05d0\u05c7\u05b8", []),
    ("hebrew_order_meteg", "\u05d0\u05b7\u05b0\u05bd", []),
    ("hebrew_order_two_sequences", "\u05d0\u05b7\u05b0\u05bd\u05d1\u05b8\u05b4\u05bd", []),
    ("hebrew_compose_across_dot", "\u05d0\u05c1\u05b7", OFF),
    ("hebrew_compose_blocked_equal", "\u05d0\u05c7\u05b8", OFF),
    ("hebrew_meteg_before_compose", "\u05d0\u05b7\u05b0\u05bd", OFF),
    ("hebrew_order_range_inside", "\u05e9\u05b8\u05c1", ["mark[1:3]=0", "mkmk[1:3]=0"]),
]]

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
        raise SystemExit("Hebrew ordering capture differs; review before --write")
    print(f"{'Captured' if args.write else 'Verified'} {len(CASES)} Hebrew ordering oracles")
