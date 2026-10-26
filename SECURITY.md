# Security Policy

The security of **FikPanel** and the servers managed by it is our top priority. We appreciate the responsible disclosure of vulnerabilities by the security community.

---

## 🛡️ Supported Versions

Only the latest active minor release receives official security patches and updates. We recommend always keeping your installation updated to the latest available release.

| Version | Supported | Status |
| :--- | :--- | :--- |
| `0.1.x` | :white_check_mark: | Currently Supported & Maintained |
| `< 0.1.0` | :x: | End of Life (Deprecated) |

---

## 🔒 Reporting a Vulnerability

If you discover a security vulnerability in FikPanel:

1. **Do NOT disclose publicly**: Please refrain from opening public GitHub issues, forum threads, or social media posts regarding unpatched vulnerabilities.
2. **Private Reporting**:
   - Preferred: Use [GitHub Private Vulnerability Reporting](https://github.com/FikFikk/fikpanel/security/advisories/new).
   - Direct Contact: Email **fikfikk.dev@gmail.com** with the subject line `[SECURITY] Vulnerability in FikPanel`.
3. **Include Details**:
   - Description of the vulnerability and its potential impact.
   - Exact FikPanel version and Linux distribution tested.
   - Step-by-step Proof of Concept (PoC) or reproducer script.
   - Any suggested mitigations or patches if available.

---

## ⏱️ Response & Remediation Process

- **Acknowledgement**: We acknowledge receipt of vulnerability reports within **48 hours**.
- **Assessment**: The core team validates and rates the severity using CVSS v3.1 scoring.
- **Fix & Patch**: A fix will be developed in a private security branch and thoroughly tested.
- **Public Disclosure**: Once a release containing the patch is deployed (e.g. via GitHub Release and `fikpanel update`), a CVE/Advisory will be published crediting the reporter (unless anonymity is requested).

---

## 🔐 Verifying Authenticity

To protect yourself against malicious tampered forks or phishing distributions:
- Always obtain FikPanel exclusively from the official GitHub repository: [github.com/FikFikk/fikpanel](https://github.com/FikFikk/fikpanel)
- Verify official binary releases with GitHub release checksums (`sha256sum`).
- Official installation scripts run directly from `https://raw.githubusercontent.com/FikFikk/fikpanel/...` or compiled binaries signed by FikFikk.
