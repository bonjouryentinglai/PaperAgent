# Cloud-first development

GitHub is Paper Agent's source of truth. Coding agents and contributors should
perform ordinary implementation, documentation, review and automated tests in a
hosted checkout. A permanent local working copy is not required.

## What runs in the cloud

- source and documentation changes
- Rust formatting and host-safe unit tests
- Node.js syntax checks, self-tests and unit tests
- shell syntax checks
- code review and pull-request preparation

Run the same gate locally or in a hosted environment with:

```sh
scripts/check.sh
```

GitHub Actions runs this command for every pull request and push. Passing it
means the change is internally consistent off-device; it does not prove that
Xochitl displayed or committed the result correctly.

## What still needs a connected Move

Use a Mac or Linux host only when a change needs:

- an ARM64 build using the Move's installed toolchain
- SSH installation, removal or diagnostics
- firmware- and Xochitl-specific integration
- physical pen, lasso, coordinate and e-ink refresh checks
- native image, new-page, Undo/Redo, reopen, export or sync acceptance

For those tasks, clone or pull the exact GitHub commit, connect the Move, then
follow `docs/DEVELOPMENT.md`. Record the tested commit, firmware and physical
checks in the pull request or release notes. Never promote an uncommitted local
device patch as the canonical version.

## Credentials

Hosted development and CI do not need the Move's ChatGPT credential. Do not add
OAuth data, device passwords or private notebook material to GitHub Actions or a
Codex cloud environment. Interactive ChatGPT login remains on the Move in Pi's
private credential store.
