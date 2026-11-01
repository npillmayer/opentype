#!/usr/bin/env python3
"""Capture/check SHAPE-012/013 oracles with HarfBuzz 15; --write replaces them."""
import argparse
import json
import shutil
from capture_harfbuzz_text import ROOT, capture

DEST = ROOT / "internal/hbcmp/testdata/text/normalization.json"
CASES = [(ident, font, "Latn", "en", "ltr", text, features)
         for ident, font, text, features in [
    ("norm_precomposed_marks", "GentiumPlus-R.ttf", "\u00e1\u0323X", []),
    ("norm_reordered_marks", "GentiumPlus-R.ttf", "a\u0301\u0323X", []),
    ("norm_canonical_marks", "GentiumPlus-R.ttf", "a\u0323\u0301X", []),
    ("norm_blocked", "GentiumPlus-R.ttf", "a\u0305\u0301X", []),
    ("norm_unblocked", "GentiumPlus-R.ttf", "a\u0316\u0301X", []),
    ("norm_equal_class", "GentiumPlus-R.ttf", "a\u0301\u0300X", []),
    ("norm_no_adot_precomposed", "ShaperTestLatin-NoADot.ttf", "\u00e1\u0323X", []),
    ("norm_no_adot_reordered", "ShaperTestLatin-NoADot.ttf", "a\u0301\u0323X", []),
    ("norm_no_adot_canonical", "ShaperTestLatin-NoADot.ttf", "a\u0323\u0301X", []),
    ("norm_partial_decomposition", "ShaperTestLatin-NoRingAcute.ttf", "\u01faX", []),
    ("norm_retain_component", "ShaperTestLatin-NoRingAcute.ttf", "\u00c5\u0301X", []),
    ("norm_retain_standalone", "ShaperTestLatin-NoRingAcute.ttf", "\u00c5X", []),
    ("norm_range_after_expansion", "ShaperTestLatin-NoEacute.ttf", "\u00e9office", ["liga=0", "liga[2:5]=1"]),
    ("norm_range_after_reordering", "GentiumPlus-R.ttf", "a\u0301\u0323office", ["liga=0", "liga[4:7]=1"]),
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
        raise SystemExit("normalization capture differs; review before --write")
    print(f"{'Captured' if args.write else 'Verified'} {len(CASES)} normalization oracles")
