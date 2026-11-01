#!/usr/bin/env python3
"""Build/check the repository-authored SHAPE-019 fixture; --write replaces it."""
import argparse
import hashlib
from io import BytesIO
from pathlib import Path

import fontTools
from fontTools.fontBuilder import FontBuilder
from fontTools.pens.ttGlyphPen import TTGlyphPen
from fontTools.feaLib.builder import addOpenTypeFeaturesFromString

DEST = Path(__file__).resolve().parents[2] / "testdata/fonts/ShaperTestLigatureMarks.ttf"
FEATURES = """
languagesystem DFLT dflt;
languagesystem latn dflt;
languagesystem hebr dflt;
markClass overline <anchor 0 0> @TOP;
markClass vertical <anchor 0 10> @TOP;
markClass extra <anchor 0 20> @TOP;
feature liga { lookupflag IgnoreMarks; sub f i by fi; } liga;
feature clig { lookupflag IgnoreMarks; sub i l by il; } clig;
feature rlig {
    lookupflag IgnoreMarks;
    sub fi l by fil;
    sub f il by fil;
} rlig;
feature ss01 { sub overline by vertical; } ss01;
feature ss02 { sub overline by vertical extra; } ss02;
feature mark {
    pos ligature fi <anchor 100 400> mark @TOP ligComponent <anchor 300 500> mark @TOP;
    pos ligature il <anchor 100 400> mark @TOP ligComponent <anchor 300 500> mark @TOP;
    pos ligature fil <anchor 100 400> mark @TOP ligComponent <anchor 300 500> mark @TOP ligComponent <anchor 500 600> mark @TOP;
} mark;
table GDEF { GlyphClassDef [f i l space], [fi il fil], [overline vertical extra], []; } GDEF;
"""

def build():
    if fontTools.__version__ != "4.61.2.dev0":
        raise ValueError("requires FontTools 4.61.2.dev0")
    names = [".notdef", "f", "i", "l", "fi", "il", "fil", "overline", "vertical", "extra", "space"]
    fb = FontBuilder(1000, isTTF=True)
    fb.setupGlyphOrder(names)
    fb.setupCharacterMap({ord("f"): "f", ord("i"): "i", ord("l"): "l", 0xFB01: "fi",
                         0x305: "overline", 0x30D: "vertical", ord(" "): "space",
                         0x5D0: "f", 0x5D1: "i", 0x5D2: "l", 0x5B0: "overline"})
    glyphs = {}
    for name in names:
        pen = TTGlyphPen(None)
        pen.moveTo((0, 0)); pen.lineTo((400, 0)); pen.lineTo((400, 600)); pen.closePath()
        glyphs[name] = pen.glyph()
    fb.setupGlyf(glyphs)
    fb.setupHorizontalMetrics({n: (600, 0) for n in names})
    fb.setupHorizontalHeader(ascent=800, descent=-200)
    fb.setupOS2(sTypoAscender=800, sTypoDescender=-200, usWinAscent=800, usWinDescent=200)
    fb.setupNameTable({"familyName": "Shaper Test Ligature Marks", "styleName": "Regular",
                       "uniqueFontIdentifier": "ShaperTestLigatureMarks-1", "fullName": "Shaper Test Ligature Marks Regular",
                       "psName": "ShaperTestLigatureMarks", "version": "Version 1.0"})
    fb.setupPost()
    addOpenTypeFeaturesFromString(fb.font, FEATURES)
    fb.font["head"].created = fb.font["head"].modified = 2082844800
    fb.font.recalcTimestamp = False
    out = BytesIO(); fb.font.save(out)
    return out.getvalue()

if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--write", action="store_true")
    args = parser.parse_args()
    data = build()
    if args.write:
        DEST.write_bytes(data)
    elif not DEST.is_file() or DEST.read_bytes() != data:
        raise SystemExit("font differs; review before --write")
    print(DEST.name, hashlib.sha256(data).hexdigest())
