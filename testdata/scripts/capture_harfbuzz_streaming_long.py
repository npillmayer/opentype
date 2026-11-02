#!/usr/bin/env python3
"""Capture/check full-text oracles for real SHAPE-015 buffer boundaries.

The compressed JSON retains every HarfBuzz output field; no repeated-unit
expectations are synthesized. Runtime tests need neither HarfBuzz nor Python.
"""
import argparse
import gzip
import json
import shutil
from capture_harfbuzz_text import ROOT, capture

DEST = ROOT / "internal/hbcmp/testdata/text/streaming_long.json.gz"
LATIN = "office AVATAR cafe\u0301 " * 600
CASES = [
    ("long_latin", "GentiumPlus-R.ttf", "Latn", "en", "ltr", LATIN, []),
    ("long_ranges", "GentiumPlus-R.ttf", "Latn", "en", "ltr", LATIN,
     ["liga=0", "liga[4095:8200]=1", "kern=1", "kern[3500:7000]=0"]),
    ("long_hebrew", "NotoSansHebrew-Regular.ttf", "Hebr", "he", "rtl", "שָׁלוֹם " * 1400, []),
    ("long_progress", "ShaperTestProgress.ttf", "Latn", "en", "ltr", "abdfi a " * 1400, ["test=1"]),
]

def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--write", action="store_true")
    parser.add_argument("--hb-shape", default="hb-shape")
    args = parser.parse_args()
    binary = shutil.which(args.hb_shape)
    if binary is None:
        parser.error("hb-shape 15.0.0 is required for capture/check")
    data = gzip.compress((json.dumps(capture(binary, CASES), ensure_ascii=False,
                                    separators=(",", ":")) + "\n").encode(), mtime=0)
    if args.write:
        DEST.write_bytes(data)
    elif not DEST.is_file() or DEST.read_bytes() != data:
        raise SystemExit("long streaming capture differs; review before --write")
    print(f"{'Captured' if args.write else 'Verified'} {len(CASES)} long streaming oracles")

if __name__ == "__main__":
    main()
