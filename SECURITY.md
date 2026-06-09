# Security Policy

## Supported versions

Clusternet Dashboard is currently in an initial MVP stage. Security fixes are applied to the latest released version.

## Reporting a vulnerability

Please report suspected vulnerabilities privately to the repository maintainers instead of opening a public issue.

When reporting, include:

- Affected version or commit
- Reproduction steps
- Impact assessment
- Any relevant logs or screenshots

## Security assumptions

- The dashboard is read-only by design.
- The browser never receives Kubernetes credentials.
- The first version does not include built-in authentication. Deploy it behind a trusted network boundary or an identity-aware proxy.
