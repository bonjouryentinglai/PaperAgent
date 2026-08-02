# Project structure

Paper Agent is the Xochitl selection integration only. The standalone Muse app
is a separate project and must not be copied into this repository.

```text
PaperAgent/
├── .github/
│   └── workflows/
│       ├── ci.yml                    # Public CI: Rust, Node, Go and shell checks
│       └── release.yml               # ARM64 bundle and desktop installer builds
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
│   ├── release/
│   │   └── install.sh                # Transactional staging, activation, rollback
│   ├── runtime/                      # Local model, Scene parser and orchestration code
│   │   ├── paper-agent-tools.ts      # Two terminating Pi tool schemas
│   │   ├── scene.mjs                 # Scene v1 validation and compilation
│   │   └── skills/                   # Fixed AI, drawing and Beautify policies
│   ├── settings/                     # AppLoad Settings QML and narrow backend
│   └── systemd/                      # On-device Paper Agent service definition
├── installer/                        # Cross-platform Wails/Go setup tool
│   ├── frontend/dist/                # Embedded installer UI
│   └── internal/                     # SSH, preflight, OAuth, release and setup logic
├── docs/
│   ├── assets/
│   │   ├── icons/                    # Original SVG menu icons and preview
│   │   └── screenshots/              # Curated public screenshots (when added)
│   ├── ARCHITECTURE.md
│   ├── COMPATIBILITY.md
│   ├── DEVELOPMENT.md
│   ├── FEATURES.md
│   ├── PERFORMANCE.md
│   └── PROJECT_STRUCTURE.md
├── scripts/                          # Login, build, install, diagnose and uninstall
├── third_party/                      # Third-party license texts
├── .gitignore
├── CODE_OF_CONDUCT.md
├── CONTRIBUTING.md
├── LICENSE
├── README.md
├── ROADMAP.md
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

`scripts/package-pi-runtime.sh` creates the deterministic ARM64 Node + Pi
runtime on an ARM64 GitHub runner. `scripts/package-release.sh` creates the
deterministic device bundle and schema-2 manifest under the ignored `dist/`
directory. The release workflow publishes both verified bundles plus the
desktop installers and creates a draft GitHub release for review. Release
output must not contain a developer's Pi credential store, SSH configuration,
private settings, notebook data, or local paths.
