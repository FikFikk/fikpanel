# Contributing to FikPanel

Thank you for your interest in contributing to **FikPanel**! We welcome contributions from the community to make modern Linux server management faster, safer, and more accessible.

---

## 📜 Intellectual Property & Authorship Notice

FikPanel is developed and maintained by **FikFikk (Fikri)** under the **GNU Affero General Public License v3.0 (AGPL-3.0)**.

To protect the integrity of the project and prevent unauthorized plagiarized clones:
1. **Attribution Preservation**: All original authorship credits, copyright banners, and project branding must remain intact.
2. **Trademark Notice**: The name "FikPanel", project logos, and visual emblems are exclusive property of FikFikk. Modifying or redistributing the code does not grant rights to use the trademark for commercial re-branding or misleading third-party projects.
3. **Developer Certificate of Origin (DCO)**: By submitting a Pull Request, you certify that your contributions are your original work or you have the legal right to submit them under AGPL-3.0.

---

## 🚀 How to Contribute

### 1. Reporting Bugs & Security Issues
- **Non-security bugs**: Search existing [GitHub Issues](https://github.com/FikFikk/fikpanel/issues) first. If not found, open an issue with:
  - Operating System version (e.g., Ubuntu 24.04 LTS, Debian 12)
  - FikPanel version (`fikpanel --version`)
  - Clear reproduction steps, logs (`journalctl -u fikpanel`), and screenshots if applicable.
- **Security vulnerabilities**: Please refer to [SECURITY.md](SECURITY.md) and do **NOT** open public issues for zero-day vulnerabilities.

### 2. Suggesting Features
- Open a feature request discussion on GitHub Issues explaining the use case, why it benefits server administrators, and potential architectural approaches.

### 3. Submitting Pull Requests (PR)

#### Development Workflow:
1. **Fork** the repository and create your feature branch:
   ```bash
   git checkout -b feat/your-feature-name
   ```
2. **Setup Local Environment**:
   - **Backend**: Go 1.22+
     ```bash
     go test ./internal/...
     ```
   - **Frontend**: Node.js 20+, npm
     ```bash
     cd web
     npm install
     npm run check
     npm run build
     ```
3. **Coding Standards**:
   - **Backend (Go)**: Clean architecture, idiomatic Go (`gofmt`), structured error handling, production-ready logging (`slog`). Code comments must be clear and technical.
   - **Frontend (Svelte)**: Follow existing Svelte 5 / TypeScript reactive patterns, responsive UI, dark-mode design system.
4. **Testing**:
   - Ensure all automated unit and integration tests pass before submitting.
5. **Commit Message Guidelines**:
   - Use [Conventional Commits](https://www.conventionalcommits.org/):
     - `feat: add docker compose stack manager`
     - `fix: resolve acme http-01 renewal timeout`
     - `docs: update api specification`
6. **Push and Open PR**:
   - Push to your fork and submit a PR to the `main` branch. Provide a concise explanation of changes and how they were tested.

---

## ⚖️ Code of Conduct

All contributors and participants agree to abide by our [Code of Conduct](CODE_OF_CONDUCT.md). Please treat fellow community members with respect and professionalism.
