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
- **FouzR/xovi-extensions**, pinned at
  `c49e4654d33e5d8b65ce1a8c5eef45454a19a999`, GPL-3.0-only. The primary-pen
  restoration and temporary palette/thickness fields in
  `device/qmd/paper-agent-selection.qmd` are adapted from its 3.27 QMD files;
  that Paper Agent QMD is therefore GPL-3.0-only. The full license is in
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
- **Pi** 0.80.7 packages, MIT, copyright 2025 Mario Zechner. Pi owns ChatGPT
  OAuth and RPC transport. Release builds redistribute the packages and their
  dependency license files inside the checksum-verified ARM64 runtime bundle.
- **Node.js** 22.22.3, distributed under the terms collected in Node.js's
  bundled `LICENSE` file. The unmodified official Linux ARM64 distribution is
  included in the checksum-verified runtime bundle.
- **AppLoad**, GPL-3.0-only, copyright AppLoad contributors. The separate
  `device/settings/backend` protocol adapter is derived from AppLoad's Rust
  backend client so the Paper Agent Settings QML can communicate with its
  narrow settings process. The full license is in `third_party/GPL-3.0.txt`.
- **remagic**, MIT, copyright 2026 Maxime. Phase 2B's desktop installer adapts
  its pure-Go device discovery, SSH, and AppLoad staging foundations. Its
  notice is preserved in `third_party/remagic-MIT.txt`.
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
- **Noto Sans Symbols 2**, SIL Open Font License 1.1, copyright The Noto
  Project Authors. It provides the generic symbol fallback used for chess,
  mathematical and other common Unicode symbols. License:
  `device/native/assets/fonts/NOTO-SYMBOLS-OFL.txt`.
- **Lucide Icons** 1.27.0, ISC with Feather-derived icons under MIT,
  copyright 2026 Lucide Icons and Contributors and 2013-present Cole Bemis.
  Six preparation and warning icons are bundled in the desktop installer.
  License: `third_party/lucide-ISC.txt`.

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
- xovi-tripletap, pinned at
  `869497aa61435448bf0077fbf75fb264dcba92c5` (GPL-3.0-only). The installer
  checksum-verifies the complete source archive before staging it as an
  external XOVI persistence service.

Paper Agent does not include smart_remarkable's GPL-2.0 prebuilt kernel
modules. Its minimal native image plugin is GPL-3.0-only and kept in a separate
source and binary boundary from the MIT Rust and JavaScript components.
