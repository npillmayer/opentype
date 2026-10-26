#!/usr/bin/env python3
"""Build/check the capability-reduced normalization fixtures.

Preparation only: requires FontTools 4.61.2.dev0. Go tests use checked-in bytes.
Default is read-only verification; --write replaces the generated files.
"""
import argparse
import hashlib
from io import BytesIO
from pathlib import Path

import fontTools
from fontTools.ttLib import TTFont

ROOT = Path(__file__).resolve().parents[2]
FONTS = ROOT / "testdata/fonts"
VERSION = "4.61.2.dev0"


def derive(source, source_hash, family, remove):
    data = (FONTS / source).read_bytes()
    if hashlib.sha256(data).hexdigest() != source_hash:
        raise ValueError(f"source font changed: {source}")
    font = TTFont(BytesIO(data), recalcTimestamp=False)
    if remove == "GPOS":
        del font["GPOS"]
    elif remove == "marks":
        gpos = font["GPOS"].table
        kept = [i for i, f in enumerate(gpos.FeatureList.FeatureRecord)
                if f.FeatureTag not in ("mark", "mkmk")]
        remap = {old: new for new, old in enumerate(kept)}
        for record in gpos.ScriptList.ScriptRecord:
            script = record.Script
            langs = [script.DefaultLangSys] + [r.LangSys for r in script.LangSysRecord]
            for lang in filter(None, langs):
                lang.FeatureIndex = [remap[i] for i in lang.FeatureIndex if i in remap]
                lang.FeatureCount = len(lang.FeatureIndex)
                lang.ReqFeatureIndex = remap.get(lang.ReqFeatureIndex, 0xFFFF)
        gpos.FeatureList.FeatureRecord = [gpos.FeatureList.FeatureRecord[i] for i in kept]
        gpos.FeatureList.FeatureCount = len(kept)
    else:
        for table in font["cmap"].tables:
            # Remove every encoding's U+00E9 entry; retain the glyph and all
            # other tables, so the missing capability is Unicode mapping.
            if table.isUnicode():
                table.cmap.pop(0x00E9, None)
    # Do not publish the derivative under the original family's reserved name.
    names = {1: family, 2: "Regular", 3: family + "-Regular-1",
             4: family + " Regular", 6: family.replace(" ", "") + "-Regular",
             16: family, 17: "Regular", 18: family + " Regular", 21: family, 22: "Regular"}
    for record in list(font["name"].names):
        if record.nameID in names:
            font["name"].setName(names[record.nameID], record.nameID,
                                 record.platformID, record.platEncID, record.langID)
    output = BytesIO()
    font.save(output)
    return output.getvalue()


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--write", action="store_true")
    args = parser.parse_args()
    if fontTools.__version__ != VERSION:
        parser.error(f"expected FontTools {VERSION}, got {fontTools.__version__}")
    outputs = {
        "ShaperTestLatin-NoEacute.ttf": derive("GentiumPlus-R.ttf",
            "fa09788adcc651d77eba7d96f8cc3a600d6484c58e3b4ff79c2b2464dbfea6f8",
            "Shaper Test Latin", "cmap"),
        "ShaperTestHebrew-NoGPOS.ttf": derive("NotoSansHebrew-Regular.ttf",
            "a7fa16fffb27bedb060a0866267c29e9859aeb9c21cc33f5b3aaf6eb062eca85",
            "Shaper Test Hebrew", "GPOS"),
        "ShaperTestHebrew-NoMark.ttf": derive("NotoSansHebrew-Regular.ttf",
            "a7fa16fffb27bedb060a0866267c29e9859aeb9c21cc33f5b3aaf6eb062eca85",
            "Shaper Test Hebrew No Mark", "marks"),
    }
    source = TTFont(FONTS / "GentiumPlus-R.ttf")
    license_text = next(n.toUnicode() for n in source["name"].names
                        if n.nameID == 13 and n.platformID == 3)
    outputs["ShaperTestLatin-LICENSE.txt"] = (license_text.replace("\r\n", "\n") + "\n").encode()
    for name, data in outputs.items():
        path = FONTS / name
        if args.write:
            path.write_bytes(data)
        elif not path.is_file() or path.read_bytes() != data:
            raise SystemExit(f"{name} differs; review before --write")
        print(name, hashlib.sha256(data).hexdigest())


if __name__ == "__main__":
    main()
