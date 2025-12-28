# Database Seeder

A development/testing utility to populate the mini-bank database with sample data.

## ⚠️ WARNING

**This tool is for DEVELOPMENT and TESTING only!**
- Uses hard-coded/predictable test data
- Includes a database cleanup option that **DELETES ALL DATA**
- **NEVER use in production environments**

## What It Does

The seeder creates:
1. **3 Test Users** with accounts and initial balances
2. **5 Random Transfers** between accounts
3. **Withdrawals** from each account

### Test Users Created

| Name | Email | Password | Initial Balance |
|------|-------|----------|----------------|
| Alice Smith | alice@example.com | password123* | $1,000.00 |
| Bob Jones | bob@example.com | password123* | $500.00 |
| Charlie Brown | charlie@example.com | password123* | $750.00 |

*Default password (configurable via `SEED_PASSWORD`)

## Usage

### Basic Seeding

```bash
# Make sure DATABASE_URL is set
export DATABASE_URL="postgresql://user:pass@localhost:5432/minibank"

# Run the seeder
go run cmd/seed/main.go
```

### With Custom Password

```bash
export SEED_PASSWORD="mySecureTestPassword123"
go run cmd/seed/main.go
```

### Clean Database Before Seeding

```bash
# WARNING: This deletes ALL existing data!
export SEED_CLEAN="true"
go run cmd/seed/main.go
```

### Combined Options

```bash
export DATABASE_URL="postgresql://user:pass@localhost:5432/minibank"
export SEED_PASSWORD="testpass123"
export SEED_CLEAN="true"
go run cmd/seed/main.go
```

## Environment Variables

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `DATABASE_URL` | Yes | - | PostgreSQL connection string |
| `SEED_PASSWORD` | No | `password123` | Password for all test users |
| `SEED_CLEAN` | No | `false` | Set to `true` to truncate all tables before seeding |

## Output Example

```
No .env file found, using environment variables
Using default password. Set SEED_PASSWORD env var to customize.
Cleaning database...
Truncated table: audit_logs
Truncated table: transactions
Truncated table: accounts
Truncated table: password_reset_tokens
Truncated table: users
Database cleaned successfully.
Seeding users...
Created user: alice@example.com (ID: 1)
Created user: bob@example.com (ID: 2)
Created user: charlie@example.com (ID: 3)
Simulating transactions...
Transferring 1234 from Account 1 to Account 2...
Transfer successful.
Transferring 2345 from Account 2 to Account 3...
Transfer successful.
...
Simulating withdrawals...
Withdrawing 567 from Account 1...
Withdrawal successful.
...
Seeding complete.
```

## Idempotency

The seeder is **idempotent** - you can run it multiple times safely:
- If users already exist, it skips creation
- Existing accounts are reused for transactions
- Use `SEED_CLEAN=true` to start completely fresh

## Testing Workflow

### Typical Development Flow

```bash
# 1. Set up environment
export DATABASE_URL="postgresql://localhost:5432/minibank_dev"

# 2. Run migrations (if needed)
# make migrate-up  # or your migration command

# 3. Seed the database
go run cmd/seed/main.go

# 4. Test your application with realistic data
go run cmd/api/main.go

# 5. When you need fresh data
SEED_CLEAN=true go run cmd/seed/main.go
```

### Integration Testing

```bash
# Clean slate for each test run
export DATABASE_URL="postgresql://localhost:5432/minibank_test"
SEED_CLEAN=true go run cmd/seed/main.go
go test ./...
```

## Notes

- **Balance Units**: All balances are in cents (100 = $1.00)
- **Transactions**: Randomly generated for realistic test data
- **Passwords**: All test users share the same password (configurable)
- **Email Format**: Uses `example.com` to avoid real email addresses

## Troubleshooting

### "DATABASE_URL is not set"
```bash
export DATABASE_URL="postgresql://user:pass@host:port/dbname"
```

### "Failed to connect to database"
- Check that PostgreSQL is running
- Verify connection string format
- Ensure database exists

### "Failed to create user: duplicate email"
- Users already exist (this is OK - seeder is idempotent)
- Or use `SEED_CLEAN=true` to start fresh

### Permission Denied on TRUNCATE
```bash
# Grant necessary permissions
psql -d minibank -c "GRANT TRUNCATE ON ALL TABLES IN SCHEMA public TO your_user;"
```

## Security Reminders

🔒 **For Development/Testing Only**
- Never use in production
- Never commit actual production credentials
- Always use test/development databases
- Clear test data when done

---

**Happy Testing! 🚀**
