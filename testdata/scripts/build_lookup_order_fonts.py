#!/usr/bin/env python3
"""Build/check repository-authored lookup-order test fonts; --write replaces them.

Requires FontTools 4.61.2.dev0 only for preparation. No upstream font is used.
The GSUB and GPOS pairs swap LookupList order; a fifth font shares an
alternate lookup across features. feaLib numbers lookups in definition order.
"""
import argparse
import hashlib
from io import BytesIO
from pathlib import Path

import fontTools
from fontTools.fontBuilder import FontBuilder
from fontTools.pens.ttGlyphPen import TTGlyphPen
from fontTools.feaLib.builder import addOpenTypeFeaturesFromString

FONTS = Path(__file__).resolve().parents[2] / "testdata/fonts"
HEADER = """
languagesystem DFLT dflt;
languagesystem latn dflt;
"""
LIGA = "feature liga { sub f i by fi; } liga;\n"
CALT = "feature calt { sub f by f.alt; } calt;\n"
KERN = "feature kern { pos i i -20; } kern;\n"
VARIANTS = {
    # liga is lookup 0, calt is lookup 1.
    "LigaFirst": HEADER + LIGA + CALT + KERN,
    # calt is lookup 0, liga is lookup 1 (control).
    "CaltFirst": HEADER + CALT + LIGA + KERN,
}

# GPOS ordering: attachment captures the base's Y placement at lookup time.
YSHIFT = "feature kern { pos f <0 50 0 0>; } kern;\n"
MARK = """markClass acute <anchor 0 0> @TOP;
feature mark { pos base f <anchor 100 200> mark @TOP; } mark;
"""
VARIANTS.update({
    "KernFirst": HEADER + LIGA + YSHIFT + MARK,
    "MarkFirst": HEADER + LIGA + MARK + YSHIFT,
    "SharedAlternates": HEADER + """
lookup ALT { sub f from [i fi f.alt]; } ALT;
feature salt { lookup ALT; } salt;
feature ss01 { lookup ALT; } ss01;
""" + KERN,
})


def build(variant):
    if fontTools.__version__ != "4.61.2.dev0":
        raise ValueError("requires FontTools 4.61.2.dev0")
    names = [".notdef", "f", "i", "fi", "f.alt"]
    fb = FontBuilder(1000, isTTF=True)
    cmap = {ord("f"): "f", ord("i"): "i"}
    if variant in ("KernFirst", "MarkFirst"):
        names.append("acute")
        cmap[0x301] = "acute"
    fb.setupGlyphOrder(names)
    fb.setupCharacterMap(cmap)
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
    fb.setupNameTable({"familyName": "Shaper Test Lookup Order " + variant, "styleName": "Regular",
                       "uniqueFontIdentifier": "ShaperTestLookupOrder-" + variant + "-1",
                       "fullName": "Shaper Test Lookup Order " + variant + " Regular",
                       "psName": "ShaperTestLookupOrder-" + variant, "version": "Version 1.0"})
    fb.setupPost()
    addOpenTypeFeaturesFromString(fb.font, VARIANTS[variant])
    fb.font["head"].created = fb.font["head"].modified = 2082844800
    fb.font.recalcTimestamp = False
    out = BytesIO()
    fb.font.save(out)
    return out.getvalue()


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--write", action="store_true")
    args = parser.parse_args()
    for variant in VARIANTS:
        data = build(variant)
        dest = FONTS / f"ShaperTestLookupOrder-{variant}.ttf"
        if args.write:
            dest.write_bytes(data)
        elif not dest.is_file() or dest.read_bytes() != data:
            raise SystemExit(f"{dest.name} differs; review before --write")
        print(dest.name, hashlib.sha256(data).hexdigest())


if __name__ == "__main__":
    main()
