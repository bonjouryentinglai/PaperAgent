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

Developer Mode and XOVI reduce the tablet's default security boundary. Users
should understand that firmware updates can disable or invalidate installed
extensions, and should keep notebook backups.
