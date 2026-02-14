#!/usr/bin/env python3
"""
repair_otf_maxp.py

CLI wrapper for repairing CFF-OTF fonts that are missing the 'maxp' table.
For CFF/CFF2 outlines, maxp must be version 0.5 and needs only numGlyphs.

Usage:
  python repair_otf_maxp.py input.otf -o output.otf
  python repair_otf_maxp.py input.otf --inplace --backup
  python repair_otf_maxp.py input.otf --stdout > fixed.otf
"""

from __future__ import annotations

import argparse
import os
import shutil
import sys
from pathlib import Path

from fontTools.ttLib import TTFont, newTable


def repair_cff_missing_maxp(
    in_path: str | os.PathLike, out_path: str | os.PathLike
) -> bool:
    """
    Repairs a CFF OTF font missing the 'maxp' table by creating maxp v0.5 with numGlyphs.

    Returns True if a repair was applied, False if no change was needed.
    """
    font = TTFont(str(in_path))

    # Sanity: this tool is for CFF/CFF2 outlines (OTF), not TrueType glyf.
    if "glyf" in font:
        raise RuntimeError(
            "This font contains 'glyf' (TrueType outlines). Use a TT maxp v1.0 repair instead."
        )
    if "CFF " not in font and "CFF2" not in font:
        raise RuntimeError(
            "Not a CFF/CFF2 OTF: neither 'CFF ' nor 'CFF2' table is present."
        )

    changed = False
    if "maxp" not in font:
        maxp = newTable("maxp")
        maxp.tableVersion = 0x00005000  # maxp v0.5 for CFF/CFF2
        maxp.numGlyphs = len(font.getGlyphOrder())
        font["maxp"] = maxp
        changed = True

    font.save(str(out_path))
    return changed


def _parse_args(argv: list[str]) -> argparse.Namespace:
    p = argparse.ArgumentParser(
        prog="repair-otf-maxp",
        description="Repair CFF/CFF2 OTF fonts missing the 'maxp' table by inserting maxp v0.5.",
    )
    p.add_argument(
        "input", help="Path to the input .otf/.ttf font file (must be CFF/CFF2 OTF)."
    )
    out_group = p.add_mutually_exclusive_group(required=False)
    out_group.add_argument("-o", "--output", help="Path to write the repaired font.")
    out_group.add_argument(
        "--inplace", action="store_true", help="Modify the input file in place."
    )
    out_group.add_argument(
        "--stdout", action="store_true", help="Write the repaired font bytes to stdout."
    )
    p.add_argument(
        "--backup",
        action="store_true",
        help="When using --inplace, create a .bak copy next to the original first.",
    )
    p.add_argument(
        "--force",
        action="store_true",
        help="Overwrite output if it already exists.",
    )
    p.add_argument(
        "--quiet",
        action="store_true",
        help="Suppress status output (errors still go to stderr).",
    )
    return p.parse_args(argv)


def main(argv: list[str] | None = None) -> int:
    args = _parse_args(sys.argv[1:] if argv is None else argv)
    in_path = Path(args.input)

    if not in_path.exists():
        print(f"Error: input not found: {in_path}", file=sys.stderr)
        return 2
    if not in_path.is_file():
        print(f"Error: input is not a file: {in_path}", file=sys.stderr)
        return 2

    # Decide output target
    if args.stdout:
        # We'll write to a temp path then stream bytes to stdout to avoid partial writes on error.
        out_path = in_path.with_suffix(in_path.suffix + ".tmp_repaired")
        cleanup_tmp = True
    elif args.inplace:
        out_path = in_path
        cleanup_tmp = False
    else:
        if not args.output:
            # Default: input.fixed.otf
            out_path = in_path.with_name(in_path.stem + ".fixed" + in_path.suffix)
        else:
            out_path = Path(args.output)
        cleanup_tmp = False

    if (
        (not args.stdout)
        and (not args.force)
        and (not args.inplace)
        and out_path.exists()
    ):
        print(
            f"Error: output already exists: {out_path} (use --force to overwrite)",
            file=sys.stderr,
        )
        return 2

    try:
        if args.inplace and args.backup:
            bak_path = in_path.with_suffix(in_path.suffix + ".bak")
            if bak_path.exists() and not args.force:
                print(
                    f"Error: backup already exists: {bak_path} (use --force to overwrite)",
                    file=sys.stderr,
                )
                return 2
            shutil.copy2(in_path, bak_path)
            if not args.quiet:
                print(f"Backup created: {bak_path}", file=sys.stderr)

        # If writing to an existing output and --force, remove it first to avoid mixed contents
        if (
            (not args.inplace)
            and (not args.stdout)
            and out_path.exists()
            and args.force
        ):
            out_path.unlink()

        changed = repair_cff_missing_maxp(in_path, out_path)

        if args.stdout:
            data = out_path.read_bytes()
            sys.stdout.buffer.write(data)
            if cleanup_tmp and out_path.exists():
                out_path.unlink()
        else:
            if not args.quiet:
                status = (
                    "repaired (maxp inserted)"
                    if changed
                    else "no change needed (maxp already present)"
                )
                print(f"{status}: {out_path}", file=sys.stderr)

        return 0

    except Exception as e:
        # Clean up temp file if stdout mode
        if args.stdout:
            try:
                if out_path.exists():
                    out_path.unlink()
            except Exception:
                pass
        print(f"Error: {e}", file=sys.stderr)
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
