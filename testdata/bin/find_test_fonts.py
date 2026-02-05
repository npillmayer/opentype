from pathlib import Path

from fontTools.ttLib import TTFont


def list_lookup_types(font_path: Path):
    f = TTFont(font_path, lazy=True)
    out = {"GSUB": [], "GPOS": []}
    for tag in ("GSUB", "GPOS"):
        if tag not in f:
            continue
        lookups = f[tag].table.LookupList.Lookup
        for i, lk in enumerate(lookups):
            out[tag].append((i, lk.LookupType))
    f.close()
    return out


def main(root: str):
    rootp = Path(root)
    fonts = []
    for ext in ("*.otf", "*.ttf", "*.ttc", "*.otc"):
        fonts += list(rootp.rglob(ext))

    # Build inverse index: (table, lookupType) -> [fonts...]
    inv = {}
    for fp in fonts:
        try:
            info = list_lookup_types(fp)
        except Exception:
            continue
        for tag in ("GSUB", "GPOS"):
            for _, t in info[tag]:
                inv.setdefault((tag, t), []).append(fp)

    # Print coverage summary
    print("== Coverage ==")
    for t in range(1, 9):
        print(f"GSUB {t}: {len(inv.get(('GSUB', t), []))} fonts")
    for t in range(1, 10):
        print(f"GPOS {t}: {len(inv.get(('GPOS', t), []))} fonts")

    # Print a few examples per type
    print("\n== Examples (up to 5 each) ==")
    for t in range(1, 9):
        fps = inv.get(("GSUB", t), [])[:5]
        print(f"GSUB {t}:")
        for fp in fps:
            print("  ", fp)
    for t in range(1, 10):
        fps = inv.get(("GPOS", t), [])[:5]
        print(f"GPOS {t}:")
        for fp in fps:
            print("  ", fp)


if __name__ == "__main__":
    # point this at a folder containing cloned repos / fonts
    main("..")
