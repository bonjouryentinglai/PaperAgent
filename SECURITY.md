# Security policy

Please report vulnerabilities through a private GitHub security advisory once
the public repository is enabled. Do not include live OAuth credentials,
notebook pages, device passwords or account identifiers in an issue.

## Security boundaries

- ChatGPT OAuth data belongs only in `/home/root/.pi/agent/auth.json` with mode
  `0600`.
- Paper Agent's Unix socket is local and owner-only.
- The native writer accepts only validated StrokeJobs and requires explicit
  confirmation.
- QML actions and runtime result kinds are strict allowlists.
- The desktop installer keeps the developer password in memory only for the
  requested SSH operation and never reads or returns Pi OAuth token values.
- Installer dependencies and Paper Agent releases use pinned or
  manifest-declared byte sizes and SHA-256 checksums over HTTPS. Device
  activation remains transactional even after desktop verification.

Developer Mode and XOVI reduce the tablet's default security boundary. Users
should understand that firmware updates can disable or invalidate installed
extensions, and should keep notebook backups.

## Notebook data and OAuth

Paper Agent sends the selected notebook region, action and prompt from the Move
to OpenAI through Pi. This is external processing: users should not select
material that they are not permitted or willing to send to an AI service.

The current `openai-codex` provider uses ChatGPT subscription OAuth and the
`chatgpt.com/backend-api/codex/responses` transport. OpenAI documents ChatGPT
sign-in for Codex products, but does not document this transport as a stable
public API for third-party applications. Treat the integration as experimental
and expect that authentication or transport changes may require a Paper Agent
update.

`scripts/uninstall.sh` selectively removes the `openai-codex` credential used
by Paper Agent while preserving any other Pi providers in the same credential
store. If no providers remain, it removes the empty file. Before transferring
or retiring a Move, also revoke the relevant ChatGPT session. Deleting the
whole file manually signs Pi out of every provider recorded there.
