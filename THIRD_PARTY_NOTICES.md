# Third-party notices

Paper Agent's original code is MIT licensed unless a file says otherwise.

## Included source and assets

- **smart_remarkable**, pinned development source
  `cb787065281b7211b012bd5e5d9be751fe5adaef`, MIT, copyright 2024-2025
  Brock Wilcox. Paper Agent adapts native pen sequencing and skeleton tracing.
  The upstream notice is preserved in `third_party/smart_remarkable-MIT.txt`.
- **remarkable-doc-links** `linkToDocumentFromSelection.qmd`, pinned
  `23e41858818ae2c1f836043852118a323cb1e77d`, file-level MIT, by Martin
  Weber and based on Mitchell Scott's `tocFromSelection.qmd`. Paper Agent's
  QMD adapts selection-menu insertion and selection-bound lookup.
- **xovi-qmd-extensions**, GPL-3.0-only. The primary-pen restoration block in
  `device/qmd/paper-agent-selection.qmd` is adapted from this project; that QMD
  file is therefore GPL-3.0-only. The full license is in
  `third_party/GPL-3.0.txt`.
- **StarNumber12046/xovi-qmd-extensions** `autoNewPage.qmd`, pinned
  `c5d0972f9a7f77dc0c1479d5b1a4d32fbb1901c4`, MIT, copyright 2025.
  Paper Agent adapts its native `documentView.addPage(document, undefined)`
  invocation. The upstream notice is preserved in
  `third_party/STARNUMBER-XOVI-QMD-EXTENSIONS-MIT.txt`.
- **remarkable-doc-links** `remarkable-xovi-native`, pinned
  `23e41858818ae2c1f836043852118a323cb1e77d`, GPL-3.0, copyright Martin
  Weber. The separate `device/xovi-image` plugin adapts its active DocumentView
  discovery and invokes Xochitl's native `insertImageFileAsSceneItem` method;
  it does not retain the upstream clipboard, inspection or notebook-link
  features. The GPL-3.0 license is preserved in
  `third_party/GPL-3.0.txt`.
- **Pi**, MIT, copyright 2025 Mario Zechner. Pi is installed as an external
  runtime and owns ChatGPT OAuth and RPC transport.
- **OpenClaw**, MIT, copyright 2026 OpenClaw Foundation, pinned reference
  `28a3540f3283b0700ffde4ffaa0a5f7303d73a09`. The image helper adapts its
  Responses image-generation request and event extraction.
- **OpenCC** `STCharacters.txt`, Apache-2.0. Its license is preserved in
  `device/native/assets/opencc/OPENCC-LICENSE.txt`.
- **Kalam**, SIL Open Font License 1.1, copyright 2014 Indian Type Foundry.
  License: `device/native/assets/fonts/KALAM-OFL.txt`.
- **ChenYuLuoyan**, SIL Open Font License 1.1. License and reserved-name terms:
  `device/native/assets/fonts/CHENYULUOYAN-OFL.txt`.
- **Open Huninn**, SIL Open Font License 1.1 with upstream attribution in
  `device/native/assets/fonts/OPEN-HUNINN-LICENSE.txt`.

Rust dependency licenses are recorded by their upstream crates, including
`ab_glyph`, `ab_glyph_rasterizer`, `owned_ttf_parser`, `ttf-parser`, `png`,
`flate2`, `fdeflate`, `crc32fast` and `libc` (MIT, Apache-2.0, or compatible
dual-license terms as declared by each crate).

## External runtime dependencies

Installation uses but does not redistribute the following binaries:

- XOVI, pinned for native-plugin generation at
  `2b99649f5e4fd6288be7792a8570bd16418adb70` (LGPL-3.0)
- rm-xovi-extensions and optional modules (GPL-3.0)
- xovi-message-broker (GPL-3.0)
- qt-resource-rebuilder (GPL-3.0)
- rm-shot (GPL-3.0)

Paper Agent does not include smart_remarkable's GPL-2.0 prebuilt kernel
modules. Its minimal native image plugin is GPL-3.0-only and kept in a separate
source and binary boundary from the MIT Rust and JavaScript components.
