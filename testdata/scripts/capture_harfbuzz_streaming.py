#!/usr/bin/env python3
"""Capture/check streaming and scope oracles; --write replaces the snapshot.

HarfBuzz sees complete text and explicit feature ranges, never our event driver.
"""
import argparse
import json
import shutil

from capture_harfbuzz_text import ROOT, capture

DEST = ROOT / "internal/hbcmp/testdata/text/streaming.json"
CASES = [
    ("stream_ligatures", "GentiumPlus-R.ttf", "Latn", "en", "ltr", "office office office", ["liga=1"]),
    ("stream_kerning", "GentiumPlus-R.ttf", "Latn", "en", "ltr", "AVATAR AVATAR", ["kern=1"]),
    ("stream_accents", "GentiumPlus-R.ttf", "Latn", "fr", "ltr", "cafe\u0301 cafe\u0301 cafe\u0301", []),
    ("stream_hebrew", "NotoSansHebrew-Regular.ttf", "Hebr", "he", "rtl", "שלום שלום שלום", []),
    ("stream_range", "GentiumPlus-R.ttf", "Latn", "en", "ltr", "office office office", ["liga=0", "liga[8:11]=1"]),
    ("scope_nested", "GentiumPlus-R.ttf", "Latn", "en", "ltr", "office office office office office", ["liga=1", "liga[0:21]=0", "liga[7:14]=1"]),
    ("scope_pair", "GentiumPlus-R.ttf", "Latn", "en", "ltr", "AVATAR", ["kern=1", "liga=1", "liga[1:2]=0"]),
    ("scope_accent", "GentiumPlus-R.ttf", "Latn", "fr", "ltr", "e\u0301 office office", ["liga=1", "liga[3:10]=0"]),
    ("progress_expand", "ShaperTestProgress.ttf", "Latn", "en", "ltr", "abababa", ["test=1"]),
    ("progress_delete", "ShaperTestProgress.ttf", "Latn", "en", "ltr", "adadada", ["test=1"]),
    ("progress_all_deleted", "ShaperTestProgress.ttf", "Latn", "en", "ltr", "dddddd", ["test=1"]),
    ("progress_contract", "ShaperTestProgress.ttf", "Latn", "en", "ltr", "afifiafifi", ["test=1"]),
]


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--write", action="store_true")
    parser.add_argument("--hb-shape", default="hb-shape")
    args = parser.parse_args()
    binary = shutil.which(args.hb_shape)
    if binary is None:
        parser.error("hb-shape 15.0.0 is required only for capture/check")
    data = (json.dumps(capture(binary, CASES, {"progress_all_deleted"}),
                       indent=2, ensure_ascii=False) + "\n").encode()
    if args.write:
        DEST.write_bytes(data)
    elif not DEST.is_file() or DEST.read_bytes() != data:
        raise SystemExit("streaming capture differs; review before --write")
    print(f"{'Captured' if args.write else 'Verified'} {len(CASES)} streaming/scope oracles")


if __name__ == "__main__":
    main()
