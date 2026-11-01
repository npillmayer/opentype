#!/usr/bin/env python3
"""Capture/check SHAPE-016 scope oracles with HarfBuzz 15; --write replaces them."""
import argparse
import json
import shutil
from capture_harfbuzz_text import ROOT, capture

DEST = ROOT / "internal/hbcmp/testdata/text/scopes.json"
CASES = [(ident, "GentiumPlus-R.ttf", "Latn", "en", "ltr", text, features)
         for ident, text, features in [
    ("scope_liga_second_off", "fi", ["liga=1", "liga[1:2]=0"]),
    ("scope_liga_first_off", "fi", ["liga=1", "liga[0:1]=0"]),
    ("scope_kern_second_off", "AV", ["kern=1", "kern[1:2]=0"]),
    ("scope_kern_first_off", "AV", ["kern=1", "kern[0:1]=0"]),
    ("scope_unrelated_ligature", "fi", ["kern=1", "kern[1:2]=0"]),
    ("scope_composition", "e\u0301x", ["liga=1", "liga[1:2]=0"]),
    ("scope_mark_inside", "x\u0301x\u0301", ["mark=1", "mark[1:3]=0"]),
    ("scope_mark_inside_no_mkmk", "x\u0301x\u0301", ["mark=1", "mkmk=0", "mark[1:3]=0"]),
    ("scope_implicit_restore", "fififi", ["liga[2:4]=0"]),
    ("scope_nested_ligatures", "fifififi", ["liga=1", "liga[0:6]=0", "liga[2:4]=1"]),
]] + [("scope_hebrew_group", "NotoSansHebrew-Regular.ttf", "Hebr", "he", "rtl", "\u05d5\u05b9", ["liga=1", "liga[1:2]=0"])]

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
        raise SystemExit("scope capture differs; review before --write")
    print(f"{'Captured' if args.write else 'Verified'} {len(CASES)} scope oracles")
