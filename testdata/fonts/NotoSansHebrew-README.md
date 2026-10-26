# Noto Sans Hebrew fixture

`NotoSansHebrew-Regular.ttf` is an unmodified static hinted font from
[notofonts/noto-fonts](https://github.com/notofonts/noto-fonts), pinned to commit
`ffebf8c1ee449e544955a7e813c54f9b73848eac` and downloaded on 2026-10-26.

- [Exact upstream font](https://raw.githubusercontent.com/notofonts/noto-fonts/ffebf8c1ee449e544955a7e813c54f9b73848eac/hinted/ttf/NotoSansHebrew/NotoSansHebrew-Regular.ttf)
- [Exact upstream license](https://raw.githubusercontent.com/notofonts/noto-fonts/ffebf8c1ee449e544955a7e813c54f9b73848eac/LICENSE), copied as
  [NotoSansHebrew-LICENSE.txt](NotoSansHebrew-LICENSE.txt): SIL Open Font License 1.1.
- Font SHA-256: `a7fa16fffb27bedb060a0866267c29e9859aeb9c21cc33f5b3aaf6eb062eca85`.

This fixture supplies cmap, metrics, GSUB/GPOS, and GDEF for the real Hebrew
[text baselines](../../internal/hbcmp/testdata/text/README.md). The existing
Gentium Plus fixture supplies the companion Latin cases. Ordinary Go tests
read the checked-in files and verify their hashes; no font download or local
font installation is required.
