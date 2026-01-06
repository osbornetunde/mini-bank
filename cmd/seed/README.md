# Database Seeder

A development/testing utility to populate the mini-bank database with sample data.

## ⚠️ WARNING

**This tool is for DEVELOPMENT and TESTING only!**
- Uses hard-coded/predictable test data
- Includes a database cleanup option that **DELETES ALL DATA**
- **NEVER use in production environments**

## What It Does

The seeder creates a **comprehensive banking environment** with:
1. **3 Test Users** with role-based permissions (Admin, Manager, User)
2. **5 Accounts** (3 primary + 2 additional savings/business accounts)
3. **Overdraft Limits** (Premium, Standard, Basic tiers)
4. **Realistic Transaction History:**
   - 5 Deposits (salaries, payments, refunds)
   - 5 Transfers (loans, gifts, invoices)
   - 5 Withdrawals (ATM, cash, teller)
5. **Multiple Accounts per User** (checking + savings/business)

### Test Users Created

| Name | Email | Password | Role | Initial Balance |
|------|-------|----------|------|----------------|
| Alice Admin | alice@example.com | password123* | Admin | $1,000.00 |
| Bob Manager | bob@example.com | password123* | Manager | $500.00 |
| Charlie User | charlie@example.com | password123* | User | $750.00 |

*Default password (configurable via `SEED_PASSWORD`)

### User Roles & Permissions

#### 1. **Admin** (alice@example.com)
Full system administrator with all permissions:
- ✅ `accounts_read`, `accounts_write`, `accounts_update`
- ✅ `transactions_read`, `transactions_process`
- ✅ `users_read`, `users_write`, `users_update`
- ✅ `permissions_manage`

**Can do:** Everything - manage users, accounts, transactions, and permissions

---

#### 2. **Manager** (bob@example.com)
Operations manager with most permissions:
- ✅ `accounts_read`, `accounts_write`, `accounts_update`
- ✅ `transactions_read`, `transactions_process`
- ✅ `users_read`
- ❌ Cannot manage permissions or modify users

**Can do:** Create/manage accounts, process transactions, view users

---

#### 3. **User** (charlie@example.com)
Standard user with read-only access:
- ✅ `accounts_read`
- ✅ `transactions_read`
- ❌ Cannot create accounts, process transactions, or manage users

**Can do:** View accounts and transactions only

### Account Types & Overdraft Limits

The seeder creates different account tiers:

| User | Account Type | Overdraft Limit | Additional Accounts |
|------|--------------|-----------------|---------------------|
| Admin | Premium | $500.00 | Savings ($2,000) |
| Manager | Standard | $200.00 | Business ($1,000) |
| User | Basic | $0.00 | None |

### Transaction Scenarios Seeded

**Deposits (5 total):**
- Salary deposits
- Freelance payments
- Cash deposits
- Refunds
- Bonuses

**Transfers (5 total):**
- Loan payments
- Gifts
- Invoice payments
- Repayments
- Shared expenses

**Withdrawals (5 total):**
- ATM withdrawals
- Cash withdrawals
- Bank teller withdrawals

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
Created user: alice@example.com (ID: 1, Role: Admin)
Assigned permissions to alice@example.com: [accounts_read accounts_write accounts_update transactions_read transactions_process users_read users_write users_update permissions_manage]
Created user: bob@example.com (ID: 2, Role: Manager)
Assigned permissions to bob@example.com: [accounts_read accounts_write accounts_update transactions_read transactions_process users_read]
Created user: charlie@example.com (ID: 3, Role: User)
Assigned permissions to charlie@example.com: [accounts_read transactions_read]
Setting overdraft limits...
Set overdraft for Account 1: Premium Account ($500 overdraft)
Set overdraft for Account 2: Standard Account ($200 overdraft)
Set overdraft for Account 3: Basic Account (no overdraft)
Simulating deposits...
Depositing 25000 cents to Account 1 (Salary deposit)...
Deposit successful: Salary deposit
Depositing 15000 cents to Account 2 (Freelance payment)...
Deposit successful: Freelance payment
...
Simulating transfers...
Transferring 5000 cents from Account 1 to Account 2 (Loan payment)...
Transfer successful: Loan payment
...
Simulating withdrawals...
Withdrawing 10000 cents from Account 1 (ATM withdrawal)...
Withdrawal successful: ATM withdrawal
...
Creating additional accounts...
Created Savings Account (ID: 4) for user 1 with balance $2000.00
Created Business Account (ID: 5) for user 2 with balance $1000.00
Seeding complete.
```

## Idempotency

The seeder is **idempotent** - you can run it multiple times safely:
- If users already exist, it skips creation and **updates their permissions**
- Existing accounts are reused for transactions
- Use `SEED_CLEAN=true` to start completely fresh

## Testing Permission-Based Access

After seeding, test the permission system with different users:

### Test Admin Access (alice@example.com)
```bash
# Login as admin
TOKEN=$(curl -s -X POST http://localhost:8080/api/v1/login \
  -H "Content-Type: application/json" \
  -d '{"email":"alice@example.com","password":"password123"}' | jq -r '.token')

# Admin can manage permissions (✅ succeeds)
curl -X PUT http://localhost:8080/api/v1/users/2/permissions \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"permissions":["accounts_read","transactions_read"]}'
```

### Test Manager Access (bob@example.com)
```bash
# Login as manager
TOKEN=$(curl -s -X POST http://localhost:8080/api/v1/login \
  -H "Content-Type: application/json" \
  -d '{"email":"bob@example.com","password":"password123"}' | jq -r '.token')

# Manager can process transactions (✅ succeeds)
curl -X POST http://localhost:8080/api/v1/transactions/transfer \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"from_account_id":1,"to_account_id":2,"amount":1000}'

# Manager cannot manage permissions (❌ fails with 403)
curl -X PUT http://localhost:8080/api/v1/users/3/permissions \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"permissions":["accounts_read"]}'
```

### Test Regular User Access (charlie@example.com)
```bash
# Login as regular user
TOKEN=$(curl -s -X POST http://localhost:8080/api/v1/login \
  -H "Content-Type: application/json" \
  -d '{"email":"charlie@example.com","password":"password123"}' | jq -r '.token')

# User can read accounts (✅ succeeds)
curl -X GET http://localhost:8080/api/v1/accounts \
  -H "Authorization: Bearer $TOKEN"

# User cannot create accounts (❌ fails with 403)
curl -X POST http://localhost:8080/api/v1/accounts \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"user_id":3,"balance":5000}'
```

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

## What's Seeded vs Not Seeded

### ✅ Fully Seeded Features
- **Users** with role-based permissions
- **Accounts** with varied balances
- **Overdraft Limits** (Premium, Standard, Basic)
- **Deposits** with realistic references
- **Transfers** between accounts
- **Withdrawals** from accounts
- **Multiple Accounts** per user
- **Transaction History** with meaningful references

### ❌ Not Seeded (Generated Automatically or Not Needed)
- **Audit Logs** - Created automatically by middleware during API calls
- **Password Reset Tokens** - Temporary, created on-demand during password reset flow
- **JWT Tokens** - Generated during login, not persisted
- **Failed Transactions** - Could be added if needed for testing error scenarios

## Notes

- **Balance Units**: All balances are in cents (100 = $1.00)
- **Transactions**: Realistic scenarios with meaningful references
- **Passwords**: All test users share the same password (configurable)
- **Email Format**: Uses `example.com` to avoid real email addresses
- **Overdrafts**: Different tiers to test overdraft functionality

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
