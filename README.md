# mini-bank

A small, modular banking application written in Go. This repository demonstrates a clean project layout with multiple storage backends (in-memory, file, Postgres), an HTTP API, and a small core domain model for accounts, transactions, and transfers.

## Table of contents
- [Quick overview](#quick-overview)
- [Features](#features)
- [Requirements](#requirements)
- [Quick start](#quick-start)
- [Configuration](#configuration)
- [Project structure](#project-structure)
- [Development notes](#development-notes)
- [Contributing](#contributing)
- [License](#license)

## Quick overview
This project is organized to separate concerns:
- `cmd/` contains the application entrypoint.
- `internal/` holds the application internals: API, core domain, storage adapters, and configuration.
- `pkg/` provides reusable packages such as logging and utilities.
- `migrations/` contains SQL migration files.

You can swap storage implementations (memory, file, postgres) without changing most of the application logic.

## Features
- Account management
- Transactions and transfers
- Multiple storage backends:
  - in-memory (`internal/storage/memory`)
  - file-based (`internal/storage/file`)
  - Postgres-backed (`internal/storage/postgres`)
- HTTP API and middleware layer under `internal/api`

## Requirements
- Go (1.18+ recommended)
- If using Postgres storage: a running PostgreSQL instance and the connection details

## Quick start

1. Build the binary:
   - `go build ./cmd/bank`
2. Run the binary:
   - `./bank` (binary name may vary by platform)
3. The server will read configuration from environment variables or the configured config loader. See the Configuration section below.

(Adjust `go` commands to your workflow — e.g., `go run ./cmd/bank` for development.)

## Configuration
Configuration values (port, database URL, etc.) are defined and loaded from the config package. See `internal/config/.env.example` for an example environment file and the exact keys the application expects.

If you run with Postgres storage, ensure your DB is migrated with the files in `migrations/` (e.g., `migrations/001_init.sql`).

## Project structure
A clean, high-level view of the repository:

banking-app/
- `cmd/`
  - `bank/`
    - `main.go` — application entrypoint
- `internal/`
  - `api/`
    - `handlers.go`
    - `middleware.go`
    - `router.go`
  - `core/`
    - `account.go`
    - `transaction.go`
    - `transfer.go`
    - `errors.go`
  - `storage/`
    - `memory/`
      - `memory_store.go`
    - `file/`
      - `file_store.go`
    - `postgres/`
      - `db.go`
      - `account_repo.go`
      - `transaction_repo.go`
      - `transfer_repo.go`
    - `storage.go` — storage abstractions and interfaces
  - `config/`
    - `config.go`
    - `.env.example`
- `pkg/`
  - `logger/`
    - `logger.go`
  - `utils/`
    - `id.go`
  - `test/`
    - `test_helpers.go`
- `migrations/`
  - `001_init.sql`
- `go.mod`
- `go.sum`
- `README.md`

## Development notes
- Prefer the storage abstraction defined in `internal/storage/storage.go` so you can swap backends for tests or runtime.
- Keep HTTP handlers thin: parse/validate input, call core services, return responses. Business rules belong in `internal/core`.
- Use the utilities in `pkg/` for consistent logging and test helpers.

## Contributing
If you want to contribute:
- Open an issue to discuss major changes.
- Keep changes small and focused; write tests for new logic.
- Follow idiomatic Go practices and format code with `gofmt`.

## Future Work / TODO
- [ ] Add tests for Account and transaction methods
- [ ] Add API handler test with httptest
- [ ] Add storage test for (in-memory & DB)
- [ ] Add concurrency-safe scheduled interest calculation
- [ ] Add WebSocket updates for account changes
- [ ] Dockerize the application
- [ ] Implement Authentication
- [ ] Add authentication middleware

## License
This project does not include a license file in the tree above. If you plan to publish or share the repository, add a `LICENSE` file and choose an appropriate license.

---
For more details, inspect the source under `internal/`, `cmd/`, and `pkg/`. If you want, I can update the README with usage examples for the API endpoints or add a sample `.env` content — tell me which storage backend you intend to use and I’ll include concrete examples.
