banking-app/
│
├── cmd/
│   └── bank/
│       └── main.go
│
├── internal/
│   ├── api/
│   │   ├── handlers.go
│   │   ├── middleware.go
│   │   └── router.go
│   │
│   ├── core/
│   │   ├── account.go
│   │   ├── transaction.go
│   │   ├── transfer.go
│   │   └── errors.go
│   │
│   ├── storage/
│   │   ├── memory/
│   │   │   └── memory_store.go
│   │   ├── file/
│   │   │   └── file_store.go
│   │   ├── postgres/
│   │   │   ├── db.go
│   │   │   ├── account_repo.go
│   │   │   ├── transaction_repo.go
│   │   │   └── transfer_repo.go
│   │   └── storage.go
│   │
│   └── config/
│       ├── config.go
│       └── .env.example
│
├── pkg/
│   ├── logger/
│   │   └── logger.go
│   ├── utils/
│   │   └── id.go
│   └── test/
│       └── test_helpers.go
│
├── migrations/
│   └── 001_init.sql
│
├── go.mod
├── go.sum
└── README.md
