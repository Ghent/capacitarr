# Contributing to Capacitarr

Thank you for your interest in contributing to Capacitarr! This document outlines the process for contributing and the legal requirements.

## Contributor License Agreement (CLA)

By submitting a merge request or otherwise contributing to this project, you agree to the following terms:

1. **License Grant**: You grant Starshadow Studios a perpetual, worldwide, non-exclusive, royalty-free, irrevocable license to use, reproduce, modify, distribute, and sublicense your contributions under any license terms, including the PolyForm Noncommercial 1.0.0 license or any successor license chosen by the project maintainers.

2. **Original Work**: You represent that your contribution is your original work and that you have the legal right to grant this license. If your employer has rights to intellectual property that you create, you represent that you have received permission to make contributions on behalf of that employer.

3. **No Warranty**: You provide your contributions on an "as is" basis, without warranties or conditions of any kind.

4. **Acknowledgment**: You acknowledge that this project is licensed under the [PolyForm Noncommercial 1.0.0](LICENSE) license and that your contributions will be subject to the same license terms.

## How to Contribute

### Reporting Issues

- Use the project's issue tracker to report bugs or request features
- Include as much detail as possible: steps to reproduce, expected behavior, actual behavior, environment details

### Submitting Changes

1. Fork the repository
2. Create a feature branch from `main` following branch naming conventions:
   - `feature/` — New features
   - `fix/` — Bug fixes
   - `refactor/` — Code refactoring
   - `docs/` — Documentation changes
   - `test/` — Test improvements
   - `chore/` — Maintenance tasks
3. Make your changes following the project's coding standards
4. Write clear, atomic commits using [Conventional Commits](https://www.conventionalcommits.org/) format:
   ```
   feat(component): add new feature
   fix(api): resolve connection timeout
   docs: update installation guide
   ```
5. Ensure all tests pass
6. Submit a merge request with a clear description of your changes

### Architecture

Capacitarr uses a layered architecture with clear separation of concerns:

- **HTTP Layer** — Thin route handlers that parse requests, call services, and return responses
- **Service Layer** — All business logic lives in `backend/internal/services/`. Each service receives a `*gorm.DB` and `*events.EventBus` via constructor injection — no global state
- **Event Bus** — A typed pub/sub system with fan-out to three subscribers: activity persister (dashboard feed), notification dispatcher (Discord/Slack), and SSE broadcaster (real-time browser updates)
- **Data Layer** — SQLite via GORM with a single baseline migration. Two purpose-specific tables: `approval_queue` (state machine) and `audit_log` (append-only history)

For the full architecture documentation with diagrams, see [docs/architecture.md](docs/architecture.md).

### Code Standards

- **Go backend**: Follow `gofmt` formatting; `golangci-lint` is run automatically via Docker
- **Vue frontend**: Follow the project's ESLint and Prettier configuration
- **Commits**: Use Conventional Commits format (required for changelog generation)
- **Documentation**: Update relevant docs when changing user-facing behavior
- **Services**: New business logic should be added to the service layer, not inline in route handlers
- **Events**: All user-visible actions should publish typed events to the event bus

### Local Development Checks

Run the full CI pipeline locally before pushing:

```bash
make ci
```

This runs lint, test, and security checks using the **same Docker images** as the GitLab CI pipeline. No additional tool installation required beyond Docker and pnpm.

Individual stages can be run separately:

```bash
make lint:ci       # golangci-lint + ESLint + Prettier format check
make test:ci       # go test + vitest
make security:ci   # govulncheck + pnpm audit
```

For auto-fixing lint and formatting issues during development:

```bash
make lint          # ESLint --fix + golangci-lint (via Docker)
make format        # Prettier --write + gofmt
```

**Full build verification via Docker:**

```bash
docker compose up --build
```

> **Note:** Do not run the backend or frontend directly for testing. Use Docker Compose to ensure the build matches the production environment.

**Recommended workflow:**

```
make lint format → make ci → commit → push
     (fix)         (verify)
```

### CI/CD Pipeline

Every push and merge request triggers a GitLab CI pipeline with these stages:

1. **Lint** — `golangci-lint` (Go), ESLint + Prettier format check (frontend)
2. **Test** — `go test` and Vitest for the frontend
3. **Build** — Docker image build verification
4. **Security** — `govulncheck` (Go) and `pnpm audit` (frontend)

The `make ci` command runs the same checks using the same Docker images, so if it passes locally it will pass in CI. Ensure all CI checks pass before requesting review.

### Merge Request Guidelines

- Keep MRs focused — one logical change per MR
- Include tests for new functionality where possible
- Update documentation if your change affects user-facing behavior
- Respond to review feedback promptly

## Questions?

If you have questions about contributing, open an issue with the `question` label.
