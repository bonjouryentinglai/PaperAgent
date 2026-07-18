# Compatibility

The current QMD hashes and geometry were developed for:

- reMarkable Paper Pro Move (`chiappa`)
- reMarkable OS 3.27.3.0
- XOVI/QMD with `xovi-message-broker`, `framebuffer-spy`, `rm-shot` and
  `qt-command-executor`

Other reMarkable models and firmware revisions are unsupported until their
selection scene, toolbar model, framebuffer geometry and Marker axes have been
measured. The Rust writer rejects non-Chiappa hardware by default.

Firmware updates can change obfuscated QML hashes even when the visible UI
looks identical. Run `scripts/doctor.sh` after every update and do not reuse an
old QMD blindly.
