# Contributing to Sailbox

Thanks for your interest in contributing. Here's how to get started.

## Getting started

1. Fork the repo and clone it locally
2. Install dependencies:
   - **Backend:** Go 1.22+, PostgreSQL
   - **Frontend:** Node.js 20+, Bun
3. Copy `.env.example` to `.env` and configure
4. Run `make dev` to start both the API and frontend

## Making changes

- Create a branch from `main`
- Keep commits focused — one change per commit
- Write clear commit messages
- Make sure `make fmt` passes before pushing
- Add or update tests if applicable

## Pull requests

- Open a PR against `main`
- Fill out the PR template
- Keep PRs small and focused — easier to review, faster to merge
- Link related issues with `Closes #123`

## Reporting bugs

Use the [bug report template](https://github.com/sailboxhq/sailbox/issues/new?template=bug_report.yml) on GitHub Issues.

## Requesting features

Use the [feature request template](https://github.com/sailboxhq/sailbox/issues/new?template=feature_request.yml) on GitHub Issues.

## License

By contributing, you agree that your contributions will be licensed under [AGPL-3.0](LICENSE).
