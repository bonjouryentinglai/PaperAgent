# Project structure

Paper Agent is the Xochitl selection integration only. The standalone Muse app
is a separate project and must not be copied into this repository.

```text
PaperAgent/
├── .github/
│   └── workflows/
│       └── ci.yml                    # Public CI: Rust, Node and shell checks
├── config/
│   └── paper-agent.env.example       # Safe, documented configuration template
├── device/
│   ├── native/                       # Bounded native renderer and input writer
│   │   ├── assets/
│   │   │   ├── fonts/                # Redistributable fonts and licenses
│   │   │   └── opencc/               # Chinese conversion data and licenses
│   │   ├── examples/                 # Non-sensitive renderer fixtures
│   │   ├── src/
│   │   ├── Cargo.lock
│   │   └── Cargo.toml
│   ├── qmd/
│   │   └── paper-agent-selection.qmd # Xochitl AI and Beautify menu actions
│   ├── runtime/                      # Local model, parser and orchestration code
│   └── systemd/                      # On-device Paper Agent service definition
├── docs/
│   ├── assets/
│   │   ├── icons/                    # Original SVG menu icons and preview
│   │   └── screenshots/              # Curated public screenshots (when added)
│   ├── ARCHITECTURE.md
│   ├── COMPATIBILITY.md
│   ├── DEVELOPMENT.md
│   ├── FEATURES.md
│   ├── PERFORMANCE.md
│   ├── PROJECT_STRUCTURE.md
│   └── ROADMAP.md
├── scripts/                          # Login, build, install, diagnose and uninstall
├── third_party/                      # Third-party license texts
├── .gitignore
├── CODE_OF_CONDUCT.md
├── CONTRIBUTING.md
├── LICENSE
├── README.md
├── SECURITY.md
└── THIRD_PARTY_NOTICES.md
```

## Generated and private files

The following belong on a developer machine or tablet, never in Git:

- `dist/`, `build/`, `.cache/` and Rust `target/` outputs
- `config/paper-agent.env` and other machine-local configuration
- ChatGPT OAuth credentials, access/refresh tokens, keys and certificates
- notebook screenshots or logs that have not been deliberately anonymized
- device IP addresses, SSH passwords, private hostnames and personal notes
- Muse source, binaries, configuration and branding

The repository's `.gitignore` covers build output and common credential file
names. Contributors must still review staged changes before every commit because
credentials can appear under unexpected filenames.

## Release layout

Release archives should be produced from tracked source by a future packaging
workflow and written under ignored `dist/`. A release should contain only the
device runtime, native binary, QMD integration, service file, installer and the
licenses required by those files. It must not contain a developer's Pi credential
store, SSH configuration or notebook data.
