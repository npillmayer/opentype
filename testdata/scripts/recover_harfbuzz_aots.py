#!/usr/bin/env python3
"""Recover the pinned AOTS cases for this repository's existing mini-fonts.

Uses only Python's standard library and a downloaded HarfBuzz source archive.
No network access, archive extraction, or shaping is performed by this script.
Pass --check to verify the recovered files without writing them.
"""

import argparse
import hashlib
import json
from pathlib import Path, PurePosixPath
import tarfile


REVISION = "29c29e1e7d1c663b9f9f8b03e980f8e7aabd1f2b"
REPOSITORY = "https://github.com/harfbuzz/harfbuzz"
UPSTREAM = "test/shape/data/aots"
ROOT = Path(__file__).resolve().parents[2]
DESTINATION = ROOT / "testdata/harfbuzz/aots"


def sha256(data):
    return hashlib.sha256(data).hexdigest()


def records(data):
    """Keep source line numbers and distinguish commented cases from active ones."""
    for number, raw in enumerate(data.decode("utf-8").splitlines(), 1):
        line = raw.strip()
        active = not line.startswith("#")
        if not active:
            line = line[1:]
        if not line.startswith("../fonts/"):
            continue
        fields = line.split(";")
        if len(fields) != 4:
            raise ValueError(f"invalid upstream record at line {number}: {raw}")
        font, options, unicodes, expected = fields
        if expected == "*":
            raise ValueError(f"record without expected output at line {number}")
        yield number, PurePosixPath(font).name, active, options


def recover(archive):
    prefix = "harfbuzz-" + REVISION + "/"
    fonts = sorted((ROOT / "testdata/fonttools").glob("*.otf"))
    if not fonts:
        raise ValueError("no repository mini-fonts found")
    font_names = {p.name for p in fonts}
    outputs, font_info, cases, sources = {}, [], [], []
    with tarfile.open(archive, "r:gz") as tar:
        def read(path):
            member = tar.getmember(prefix + path)
            if not member.isfile():
                raise ValueError(f"not a regular archive file: {path}")
            return tar.extractfile(member).read()

        for font in fonts:
            path = f"{UPSTREAM}/fonts/{font.name}"
            original, local = read(path), font.read_bytes()
            if original != local:
                raise ValueError(f"font differs from pinned upstream: {font.name}")
            font_info.append({
                "local_path": font.relative_to(ROOT).as_posix(),
                "upstream_path": path,
                "sha256": sha256(local),
                "byte_identical": True,
            })

        registered = read(f"{UPSTREAM}/meson.build").decode("utf-8")
        paths = sorted(m.name[len(prefix):] for m in tar.getmembers()
                       if m.isfile() and m.name.startswith(prefix + UPSTREAM + "/tests/")
                       and m.name.endswith(".tests"))
        for path in paths:
            data = read(path)
            rows = list(records(data))
            if not any(font in font_names for _, font, _, _ in rows):
                continue
            name = PurePosixPath(path).name
            if f"'{name}'" not in registered:
                raise ValueError(f"test file not registered upstream: {name}")
            relative = "tests/" + name
            outputs[relative] = data
            sources.append({"local_path": relative, "upstream_path": path,
                            "sha256": sha256(data)})
            for line, font, active, options in rows:
                present = font in font_names
                cases.append({
                    "id": f"{name}:{line}",
                    "source_file": relative,
                    "source_line": line,
                    "font": font,
                    "local_font_path": f"testdata/fonttools/{font}" if present else None,
                    "upstream_active": active,
                    "recovery_status": ("candidate" if active else "upstream_disabled")
                                       if present else "font_not_in_local_subset",
                    "expected_fields": "glyph_ids" if "--no-positions" in options
                                       else "glyph_ids_and_origins",
                })

        for path, relative in [(f"{UPSTREAM}/COPYING", "COPYING"),
                               ("COPYING", "COPYING.harfbuzz")]:
            data = read(path)
            outputs[relative] = data
            sources.append({"local_path": relative, "upstream_path": path,
                            "sha256": sha256(data)})

    matched = {c["font"] for c in cases if c["local_font_path"]}
    if matched != font_names:
        raise ValueError(f"fonts without upstream records: {sorted(font_names - matched)}")
    counts = {status: sum(c["recovery_status"] == status for c in cases)
              for status in ("candidate", "upstream_disabled", "font_not_in_local_subset")}
    manifest = {
        "schema_version": 1,
        "upstream_repository": REPOSITORY,
        "upstream_revision": REVISION,
        "archive_url": f"https://codeload.github.com/harfbuzz/harfbuzz/tar.gz/{REVISION}",
        "summary": {"fonts_verified": len(fonts),
                    "test_files": sum(p.endswith(".tests") for p in outputs),
                    "records": len(cases), **counts},
        "fonts": font_info,
        "source_files": sources,
        "cases": cases,
    }
    outputs["manifest.json"] = (json.dumps(manifest, indent=2) + "\n").encode("utf-8")
    return outputs, manifest["summary"]


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("archive", type=Path, help="source tar.gz for the pinned revision")
    parser.add_argument("--check", action="store_true", help="verify without writing files")
    args = parser.parse_args()
    outputs, summary = recover(args.archive)
    for relative, data in outputs.items():
        path = DESTINATION / relative
        if args.check:
            if not path.is_file() or path.read_bytes() != data:
                raise SystemExit(f"recovery mismatch: {path.relative_to(ROOT)}")
        else:
            path.parent.mkdir(parents=True, exist_ok=True)
            path.write_bytes(data)
    print(("Verified" if args.check else "Recovered") + ": " + json.dumps(summary))


if __name__ == "__main__":
    main()
