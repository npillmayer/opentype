#!/usr/bin/env python3
"""Build/check a repository-authored progress-test font; --write replaces it.

Requires FontTools 4.61.2.dev0 only for preparation. No upstream font is used.
"""
import argparse
import hashlib
from io import BytesIO
from pathlib import Path

import fontTools
from fontTools.fontBuilder import FontBuilder
from fontTools.pens.ttGlyphPen import TTGlyphPen
from fontTools.feaLib.builder import addOpenTypeFeaturesFromString

DEST = Path(__file__).resolve().parents[2] / "testdata/fonts/ShaperTestProgress.ttf"
FEATURES = """
languagesystem DFLT dflt;
languagesystem latn dflt;
feature test {
    sub b by b b;
    sub d by NULL;
    sub f i by fi;
} test;
feature kern { pos a a -20; } kern;
"""


def build():
    if fontTools.__version__ != "4.61.2.dev0":
        raise ValueError("requires FontTools 4.61.2.dev0")
    names = [".notdef", "a", "b", "d", "f", "i", "fi", "space"]
    fb = FontBuilder(1000, isTTF=True)
    fb.setupGlyphOrder(names)
    fb.setupCharacterMap({ord(c): c for c in "abdfi"} | {32: "space"})
    glyphs = {}
    for name in names:
        pen = TTGlyphPen(None)
        pen.moveTo((0, 0))
        pen.lineTo((400, 0))
        pen.lineTo((400, 600))
        pen.lineTo((0, 600))
        pen.closePath()
        glyphs[name] = pen.glyph()
    fb.setupGlyf(glyphs)
    fb.setupHorizontalMetrics({n: (600, 0) for n in names})
    fb.setupHorizontalHeader(ascent=800, descent=-200)
    fb.setupOS2(sTypoAscender=800, sTypoDescender=-200, usWinAscent=800, usWinDescent=200)
    fb.setupNameTable({"familyName": "Shaper Test Progress", "styleName": "Regular",
                       "uniqueFontIdentifier": "ShaperTestProgress-1",
                       "fullName": "Shaper Test Progress Regular",
                       "psName": "ShaperTestProgress-Regular", "version": "Version 1.0"})
    fb.setupPost()
    addOpenTypeFeaturesFromString(fb.font, FEATURES)
    fb.font["head"].created = fb.font["head"].modified = 2082844800
    fb.font.recalcTimestamp = False
    out = BytesIO()
    fb.font.save(out)
    return out.getvalue()


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--write", action="store_true")
    args = parser.parse_args()
    data = build()
    if args.write:
        DEST.write_bytes(data)
    elif not DEST.is_file() or DEST.read_bytes() != data:
        raise SystemExit("progress font differs; review before --write")
    print(DEST.name, hashlib.sha256(data).hexdigest())


if __name__ == "__main__":
    main()
