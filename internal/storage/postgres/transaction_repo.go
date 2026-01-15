package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"mini-bank/internal/core"
	"mini-bank/internal/storage"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/lib/pq"
)

// Ensure our repo implements storage.Storage partially (we'll implement methods we need)
var _ storage.Storage = (*Repo)(nil)

type Repo struct {
	db *DB
}

func NewRepo(db *DB) *Repo {
	return &Repo{db: db}
}

// CreateAccount creates a new account
func (r *Repo) CreateAccount(ctx context.Context, userID int, balance int64) (*core.Account, error) {
	const q = `INSERT INTO accounts (user_id, balance, overdraft_limit) VALUES ($1, $2, 0) RETURNING id, user_id, balance, overdraft_limit, created_at`
	row := r.db.QueryRowContext(ctx, q, userID, balance)
	return scanAccount(row)
}

// Helper to scan account
func scanAccount(row scanner) (*core.Account, error) {
	var a core.Account
	if err := row.Scan(&a.ID, &a.UserID, &a.Balance, &a.OverdraftLimit, &a.CreatedAt); err != nil {
		return nil, err
	}
	return &a, nil
}

func scanTransaction(row scanner) (*core.Transaction, error) {
	var t core.Transaction
	if err := row.Scan(&t.ID, &t.AccountID, &t.Type, &t.Amount, &t.Reference, &t.FromAccountID, &t.ToAccountID, &t.Timestamp, &t.FeeAmount, &t.ParentTransactionID, &t.IsFeeTransaction); err != nil {
		return nil, err
	}
	return &t, nil
}

func scanUser(row scanner) (*core.User, error) {
	var u core.User
	if err := row.Scan(&u.ID, &u.FirstName, &u.LastName, &u.Email, &u.Balance, pq.Array(&u.Permissions)); err != nil {
		return nil, err
	}
	return &u, nil
}

type scanner interface {
	Scan(dest ...any) error
}

// GetAccount retrieves an account by id
func (r *Repo) GetAccount(ctx context.Context, id int) (*core.Account, error) {
	const q = `SELECT id, user_id, balance, overdraft_limit, created_at FROM accounts WHERE id = $1`
	row := r.db.QueryRowContext(ctx, q, id)
	acc, err := scanAccount(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, storage.ErrAccountNotFound
		}
		return nil, err
	}
	return acc, nil
}

// ListAccounts returns all accounts
func (r *Repo) ListAccounts(ctx context.Context) ([]*core.Account, error) {
	const q = `SELECT id, user_id, balance, overdraft_limit, created_at FROM accounts ORDER BY id`
	rows, err := r.db.QueryContext(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var res []*core.Account
	for rows.Next() {
		a, err := scanAccount(rows)
		if err != nil {
			return nil, err
		}
		res = append(res, a)
	}
	return res, rows.Err()
}

// Deposit performs an atomic deposit and returns the updated account.
func (r *Repo) Deposit(ctx context.Context, accountID int, amount int64, reference string) (*core.Account, error) {
	if amount <= 0 {
		return nil, errors.New("amount must be positive")
	}

	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	// Update balance and return account details
	const upd = `UPDATE accounts SET balance = balance + $1 WHERE id = $2 RETURNING id, user_id, balance, overdraft_limit, created_at`
	var acc core.Account
	if err := tx.QueryRowContext(ctx, upd, amount, accountID).Scan(&acc.ID, &acc.UserID, &acc.Balance, &acc.OverdraftLimit, &acc.CreatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, storage.ErrAccountNotFound
		}
		return nil, err
	}

	// Insert transaction
	const ins = `INSERT INTO transactions (account_id, type, amount, reference, created_at) VALUES ($1, $2, $3, $4, $5)`
	if _, err := tx.ExecContext(ctx, ins, accountID, "deposit", amount, nullIfEmpty(reference), time.Now().UTC()); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return &acc, nil
}

// Withdraw performs an atomic withdrawal and returns the updated account.
func (r *Repo) Withdraw(ctx context.Context, accountID int, amount int64, reference string, feeAmount int64) (*core.Account, error) {
	if amount <= 0 {
		return nil, errors.New("amount must be positive")
	}

	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	// Deduct total amount (amount + fee) from account
	totalDeduction := amount + feeAmount

	// Attempt to debit if sufficient funds exist; RETURNING gives new account details
	const debit = `UPDATE accounts SET balance = balance - $1 WHERE id = $2 AND balance + overdraft_limit >= $1 RETURNING id, user_id, balance, overdraft_limit, created_at`
	var acc core.Account
	if err := tx.QueryRowContext(ctx, debit, totalDeduction, accountID).Scan(&acc.ID, &acc.UserID, &acc.Balance, &acc.OverdraftLimit, &acc.CreatedAt); err != nil {
		if err == sql.ErrNoRows {
			// The atomic update failed. Let's find out why.
			var exists bool
			// Check if the account exists at all.
			err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM accounts WHERE id=$1)`, accountID).Scan(&exists)
			if err != nil {
				return nil, err // A different database error occurred
			}
			if !exists {
				return nil, storage.ErrAccountNotFound
			}
			return nil, storage.ErrInsufficientFunds
		}
		return nil, err
	}

	// Insert main withdrawal transaction with fee_amount
	const ins = `INSERT INTO transactions (account_id, type, amount, reference, fee_amount, is_fee_transaction, created_at) VALUES ($1, $2, $3, $4, $5, FALSE, $6) RETURNING id`
	var transactionID int64
	if err := tx.QueryRowContext(ctx, ins, accountID, "withdraw", amount, nullIfEmpty(reference), feeAmount, time.Now().UTC()).Scan(&transactionID); err != nil {
		return nil, err
	}

	// If there's a fee, create a separate fee transaction
	if feeAmount > 0 {
		feeReference := fmt.Sprintf("%s-fee", reference)
		const feeIns = `INSERT INTO transactions (account_id, type, amount, reference, parent_transaction_id, fee_amount, is_fee_transaction, created_at) VALUES ($1, 'fee', $2, $3, $4, 0, TRUE, $5)`
		if _, err := tx.ExecContext(ctx, feeIns, accountID, feeAmount, feeReference, transactionID, time.Now().UTC()); err != nil {
			return nil, fmt.Errorf("failed to insert fee transaction: %w", err)
		}
	}

	// commit
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return &acc, nil
}

// RecordTransaction is a more generic method to append a transaction to the log.
// It's primarily intended for multi-account operations like transfers, where balance
// updates are handled separately within a single database transaction.
func (r *Repo) RecordTransaction(ctx context.Context, txn *core.Transaction) error {
	const ins = `INSERT INTO transactions (account_id, type, amount, reference, from_account_id, to_account_id, created_at) VALUES ($1,$2,$3,$4,$5,$6,$7)`
	_, err := r.db.ExecContext(ctx, ins, txn.AccountID, txn.Type, txn.Amount, nullIfEmpty(txn.Reference),
		nullInt(txn.FromAccountID), nullInt(txn.ToAccountID), txn.Timestamp)
	return err
}

// ListTransactions returns transactions for an account
func (r *Repo) ListTransactions(ctx context.Context, accountID int) ([]*core.Transaction, error) {
	const q = `SELECT id, account_id, type, amount, reference, from_account_id, to_account_id, created_at, fee_amount, parent_transaction_id, is_fee_transaction FROM transactions WHERE account_id = $1 ORDER BY created_at DESC`
	rows, err := r.db.QueryContext(ctx, q, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var res []*core.Transaction
	for rows.Next() {
		var t core.Transaction
		var from sql.NullInt64
		var to sql.NullInt64
		var ref sql.NullString
		var parentTxID sql.NullInt64
		if err := rows.Scan(&t.ID, &t.AccountID, &t.Type, &t.Amount, &ref, &from, &to, &t.Timestamp, &t.FeeAmount, &parentTxID, &t.IsFeeTransaction); err != nil {
			return nil, err
		}
		if ref.Valid {
			t.Reference = ref.String
		}
		if from.Valid {
			v := int(from.Int64)
			t.FromAccountID = &v
		}
		if to.Valid {
			v := int(to.Int64)
			t.ToAccountID = &v
		}
		if parentTxID.Valid {
			v := parentTxID.Int64
			t.ParentTransactionID = &v
		}
		res = append(res, &t)
	}
	return res, rows.Err()
}

// ListTransactionsPaginated returns paginated transactions for an account with optional filters
func (r *Repo) ListTransactionsPaginated(ctx context.Context, accountID int, filters storage.TransactionFilters, pagination storage.PaginationParams) (*storage.PaginatedResult, error) {
	// Build WHERE clause dynamically
	whereClause := "WHERE account_id = $1"
	args := []interface{}{accountID}
	argCount := 1

	// Add optional filters
	if filters.Status != "" {
		argCount++
		whereClause += fmt.Sprintf(" AND type = $%d", argCount)
		args = append(args, filters.Status)
	}

	if filters.Reference != "" {
		argCount++
		whereClause += fmt.Sprintf(" AND reference = $%d", argCount)
		args = append(args, filters.Reference)
	}

	if filters.DateFrom != nil {
		argCount++
		whereClause += fmt.Sprintf(" AND created_at >= $%d", argCount)
		args = append(args, filters.DateFrom)
	}

	if filters.DateTo != nil {
		argCount++
		whereClause += fmt.Sprintf(" AND created_at <= $%d", argCount)
		args = append(args, filters.DateTo)
	}

	// Get total count with filters
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM transactions %s", whereClause)
	var totalCount int64
	err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&totalCount)
	if err != nil {
		return nil, fmt.Errorf("failed to count transactions: %w", err)
	}

	// Get paginated results
	argCount++
	limitPlaceholder := fmt.Sprintf("$%d", argCount)
	args = append(args, pagination.Limit)

	argCount++
	offsetPlaceholder := fmt.Sprintf("$%d", argCount)
	args = append(args, pagination.Offset)

	query := fmt.Sprintf(`
		SELECT id, account_id, type, amount, reference, from_account_id, to_account_id, created_at, fee_amount, parent_transaction_id, is_fee_transaction
		FROM transactions
		%s
		ORDER BY created_at DESC
		LIMIT %s OFFSET %s
	`, whereClause, limitPlaceholder, offsetPlaceholder)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list transactions: %w", err)
	}
	defer rows.Close()

	var transactions []*core.Transaction
	for rows.Next() {
		var t core.Transaction
		var from sql.NullInt64
		var to sql.NullInt64
		var ref sql.NullString
		var parentTxID sql.NullInt64
		if err := rows.Scan(&t.ID, &t.AccountID, &t.Type, &t.Amount, &ref, &from, &to, &t.Timestamp, &t.FeeAmount, &parentTxID, &t.IsFeeTransaction); err != nil {
			return nil, fmt.Errorf("failed to scan transaction: %w", err)
		}
		if ref.Valid {
			t.Reference = ref.String
		}
		if from.Valid {
			v := int(from.Int64)
			t.FromAccountID = &v
		}
		if to.Valid {
			v := int(to.Int64)
			t.ToAccountID = &v
		}
		if parentTxID.Valid {
			v := parentTxID.Int64
			t.ParentTransactionID = &v
		}
		transactions = append(transactions, &t)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating transactions: %w", err)
	}

	return &storage.PaginatedResult{
		Transactions: transactions,
		TotalCount:   totalCount,
	}, nil
}

// UpdateBalance updates an account's balance.
func (r *Repo) UpdateBalance(ctx context.Context, id int, newBalance int64) error {
	const q = `UPDATE accounts SET balance = $1 WHERE id = $2`
	_, err := r.db.ExecContext(ctx, q, newBalance, id)
	return err
}

// UpdateOverdraftLimit updates an account's overdraft limit and returns the updated account.
func (r *Repo) UpdateOverdraftLimit(ctx context.Context, accountID int, newLimit int64) (*core.Account, error) {
	const q = `UPDATE accounts SET overdraft_limit = $1 WHERE id = $2 RETURNING id, user_id, balance, overdraft_limit, created_at`
	row := r.db.QueryRowContext(ctx, q, newLimit, accountID)
	acc, err := scanAccount(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, storage.ErrAccountNotFound
		}
		return nil, err
	}
	return acc, nil
}

// Transfer performs a transactional transfer between two accounts with optional fee.
func (r *Repo) Transfer(ctx context.Context, fromID, toID int, amount int64, reference string, feeAmount int64) (*core.Account, *core.Account, error) {
	if amount <= 0 {
		return nil, nil, errors.New("amount must be positive")
	}

	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{
		Isolation: sql.LevelSerializable,
	})
	if err != nil {
		return nil, nil, err
	}
	defer tx.Rollback()

	// Deduct total amount (amount + fee) from sender
	totalDeduction := amount + feeAmount

	// Withdraw from sender
	const debit = `UPDATE accounts SET balance = balance - $1 WHERE id = $2 AND balance + overdraft_limit >= $1 RETURNING id, user_id, balance, overdraft_limit, created_at`
	var fromAcc core.Account
	if err := tx.QueryRowContext(ctx, debit, totalDeduction, fromID).Scan(&fromAcc.ID, &fromAcc.UserID, &fromAcc.Balance, &fromAcc.OverdraftLimit, &fromAcc.CreatedAt); err != nil {
		if err == sql.ErrNoRows {
			// This could mean insufficient funds or the account doesn't exist.
			// A more robust implementation could check for existence first.
			return nil, nil, storage.ErrInsufficientFunds
		}
		return nil, nil, err
	}

	// Deposit to receiver (only the transfer amount, not the fee)
	const credit = `UPDATE accounts SET balance = balance + $1 WHERE id = $2 RETURNING id, user_id, balance, overdraft_limit, created_at`
	var toAcc core.Account
	if err := tx.QueryRowContext(ctx, credit, amount, toID).Scan(&toAcc.ID, &toAcc.UserID, &toAcc.Balance, &toAcc.OverdraftLimit, &toAcc.CreatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil, storage.ErrAccountNotFound
		}
		return nil, nil, err
	}

	// Record transaction for sender with fee_amount
	const insFrom = `INSERT INTO transactions (account_id, type, amount, to_account_id, reference, fee_amount, is_fee_transaction, created_at) VALUES ($1, 'transfer', $2, $3, $4, $5, FALSE, $6) RETURNING id`
	senderRef := reference + "-from"
	var senderTransactionID int64
	if err := tx.QueryRowContext(ctx, insFrom, fromID, amount, toID, nullIfEmpty(senderRef), feeAmount, time.Now().UTC()).Scan(&senderTransactionID); err != nil {
		return nil, nil, err
	}

	// Record transaction for receiver
	const insTo = `INSERT INTO transactions (account_id, type, amount, from_account_id, reference, fee_amount, is_fee_transaction, created_at) VALUES ($1, 'transfer', $2, $3, $4, 0, FALSE, $5)`
	receiverRef := reference + "-to"
	if _, err := tx.ExecContext(ctx, insTo, toID, amount, fromID, nullIfEmpty(receiverRef), time.Now().UTC()); err != nil {
		return nil, nil, err
	}

	// If there's a fee, create a separate fee transaction for the sender
	if feeAmount > 0 {
		feeReference := fmt.Sprintf("%s-fee", reference)
		const feeIns = `INSERT INTO transactions (account_id, type, amount, reference, parent_transaction_id, fee_amount, is_fee_transaction, created_at) VALUES ($1, 'fee', $2, $3, $4, 0, TRUE, $5)`
		if _, err := tx.ExecContext(ctx, feeIns, fromID, feeAmount, feeReference, senderTransactionID, time.Now().UTC()); err != nil {
			return nil, nil, fmt.Errorf("failed to insert fee transaction: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, nil, err
	}

	return &fromAcc, &toAcc, nil
}

// Payment performs a deposit or withdrawal and returns the updated account.
// Note: This is a legacy method that doesn't support fees. For fee-enabled withdrawals,
// use the Withdraw method directly from the service layer.
func (r *Repo) Payment(ctx context.Context, accountID int, amount int64, paymentType storage.PaymentType, reference string) (*core.Account, error) {
	switch paymentType {
	case storage.Deposit:
		return r.Deposit(ctx, accountID, amount, reference)
	case storage.Withdraw:
		// Payment method doesn't charge fees - pass 0 for feeAmount
		return r.Withdraw(ctx, accountID, amount, reference, 0)
	default:
		return nil, fmt.Errorf("unknown payment type: %s", paymentType)
	}
}

func (r *Repo) GetTransaction(ctx context.Context, ref string) (*core.Transaction, error) {
	const q = `SELECT id, account_id, type, amount, reference, from_account_id, to_account_id, created_at, fee_amount, parent_transaction_id, is_fee_transaction FROM transactions WHERE reference = $1`

	row := r.db.QueryRowContext(ctx, q, ref)
	trx, err := scanTransaction(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, storage.ErrTransactionNotFound
		}
		return nil, err
	}

	return trx, nil
}

func (r *Repo) CreateUser(ctx context.Context, firstName string, lastName string, email string, password string) (*core.User, error) {
	const ins = `INSERT INTO users (first_name, last_name, email, password, permissions) VALUES ($1, $2, $3, $4, $5) RETURNING id`

	var id int
	defaultPermissions := []string{}

	row := r.db.QueryRowContext(ctx, ins, firstName, lastName, email, password, pq.Array(defaultPermissions))
	if err := row.Scan(&id); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			switch pgErr.ConstraintName {
			case "users_email_key":
				return nil, storage.ErrDuplicateEmail
			default:
				return nil, errors.New("duplicate entry")
			}
		}
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	return &core.User{ID: id, FirstName: firstName, LastName: lastName, Email: email, Permissions: defaultPermissions}, nil
}

func (r *Repo) CreateUserWithAccount(ctx context.Context, firstName string, lastName string, email string, password string, initialBalance int64) (*core.User, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	const insUser = `INSERT INTO users (first_name, last_name, email, password, permissions) VALUES ($1, $2, $3, $4, $5) RETURNING id`
	var userID int
	defaultPermissions := []string{}
	if err := tx.QueryRowContext(ctx, insUser, firstName, lastName, email, password, pq.Array(defaultPermissions)).Scan(&userID); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, storage.ErrDuplicateEmail
		}
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	const insAccount = `INSERT INTO accounts (user_id, balance, overdraft_limit) VALUES ($1, $2, 0) RETURNING id, balance, overdraft_limit`
	var accID int
	var balance int64
	var odLimit int64
	if err := tx.QueryRowContext(ctx, insAccount, userID, initialBalance).Scan(&accID, &balance, &odLimit); err != nil {
		return nil, fmt.Errorf("failed to create account for user: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return &core.User{
		ID:          userID,
		FirstName:   firstName,
		LastName:    lastName,
		Email:       email,
		Balance:     &balance,
		Permissions: defaultPermissions,
	}, nil
}

func (r *Repo) GetUsers(ctx context.Context, pagination storage.PaginationParams) (*storage.UsersPaginatedResult, error) {
	// Use CTE with window function to get total count and paginated results in a single query
	q := `
		WITH paginated_users AS (
			SELECT id, first_name, last_name, email, permissions,
			       COUNT(*) OVER() as total_count
			FROM users
			ORDER BY id
			LIMIT $1 OFFSET $2
		)
		SELECT
			u.id, u.first_name, u.last_name, u.email, u.permissions, u.total_count,
			a.id, a.balance, a.overdraft_limit, a.created_at
		FROM paginated_users u
		LEFT JOIN accounts a ON u.id = a.user_id
		ORDER BY u.id, a.id
	`
	rows, err := r.db.QueryContext(ctx, q, pagination.Limit, pagination.Offset)
	if err != nil {
		return nil, fmt.Errorf("failed to query users: %w", err)
	}
	defer rows.Close()

	userMap := make(map[int]*core.User)
	var users []*core.User
	var totalCount int64

	for rows.Next() {
		var userID int
		var firstName, lastName, email string
		var permissions []string
		var rowTotalCount int64

		var accountID sql.NullInt64
		var balance sql.NullInt64
		var overdraftLimit sql.NullInt64
		var createdAt sql.NullTime

		if err := rows.Scan(
			&userID, &firstName, &lastName, &email, pq.Array(&permissions), &rowTotalCount,
			&accountID, &balance, &overdraftLimit, &createdAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan user row: %w", err)
		}

		// Total count is the same for all rows, capture it once
		if totalCount == 0 {
			totalCount = rowTotalCount
		}

		user, exists := userMap[userID]
		if !exists {
			user = &core.User{
				ID:          userID,
				FirstName:   firstName,
				LastName:    lastName,
				Email:       email,
				Permissions: permissions,
				Balance:     nil, // Calculated via domain method
				Accounts:    []*core.Account{},
			}
			users = append(users, user)
			userMap[userID] = user
		}

		if accountID.Valid {
			acc := &core.Account{
				ID:             int(accountID.Int64),
				UserID:         userID,
				Balance:        balance.Int64,
				OverdraftLimit: overdraftLimit.Int64,
				CreatedAt:      createdAt.Time,
			}
			user.Accounts = append(user.Accounts, acc)
		}
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating user rows: %w", err)
	}

	return &storage.UsersPaginatedResult{
		Users:      users,
		TotalCount: totalCount,
	}, nil
}

func (r *Repo) GetUser(ctx context.Context, userId int) (*core.User, error) {
	q := `
		SELECT u.id, u.first_name, u.last_name, u.email, u.permissions,
		       COALESCE(SUM(a.balance), 0) as total_balance
		FROM users u
		LEFT JOIN accounts a ON u.id = a.user_id
		WHERE u.id = $1
		GROUP BY u.id, u.first_name, u.last_name, u.email, u.permissions
	`
	row := r.db.QueryRowContext(ctx, q, userId)

	var u core.User
	var totalBalance int64
	if err := row.Scan(&u.ID, &u.FirstName, &u.LastName, &u.Email, pq.Array(&u.Permissions), &totalBalance); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, storage.ErrUserNotFound
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}
	u.Balance = &totalBalance
	return &u, nil
}

func (r *Repo) UpdateUser(ctx context.Context, id int, firstName, lastName, email string) (*core.User, error) {
	// First update the user
	updateQ := `UPDATE users SET first_name = $2, last_name = $3, email = $4 WHERE id = $1`
	result, err := r.db.ExecContext(ctx, updateQ, id, firstName, lastName, email)
	if err != nil {
		return nil, fmt.Errorf("failed to update user: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return nil, storage.ErrUserNotFound
	}

	// Then fetch the updated user with aggregated balance
	return r.GetUser(ctx, id)
}

func (r *Repo) DeleteUser(ctx context.Context, id int) error {
	q := `DELETE FROM users WHERE id = $1`

	result, err := r.db.ExecContext(ctx, q, id)
	if err != nil {
		return fmt.Errorf("failed to delete user: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return storage.ErrUserNotFound
	}
	return nil
}

func (r *Repo) GetUserByEmail(ctx context.Context, email string) (*core.User, error) {
	q := `SELECT u.id, u.first_name, u.last_name, u.email, a.balance, u.permissions, u.password FROM users u INNER JOIN accounts a ON u.id = a.user_id WHERE u.email = $1`

	row := r.db.QueryRowContext(ctx, q, email)
	var user core.User
	var password string

	if err := row.Scan(&user.ID, &user.FirstName, &user.LastName, &user.Email, &user.Balance, pq.Array(&user.Permissions), &password); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, storage.ErrUserNotFound
		}
		return nil, fmt.Errorf("failed to get user by email: %w", err)
	}
	user.Password = &password
	return &user, nil
}

func (r *Repo) CreatePasswordResetToken(ctx context.Context, userID int, token string, expiresAt time.Time) error {
	const ins = `INSERT INTO password_reset_tokens (user_id, token, expires_at) VALUES ($1, $2, $3)`
	_, err := r.db.ExecContext(ctx, ins, userID, token, expiresAt)
	return err
}

func (r *Repo) GetPasswordResetToken(ctx context.Context, token string) (userID int, expiresAt time.Time, usedAt *time.Time, err error) {
	const q = `SELECT user_id, expires_at, used_at FROM password_reset_tokens WHERE token = $1`
	var nullUsedAt sql.NullTime
	err = r.db.QueryRowContext(ctx, q, token).Scan(&userID, &expiresAt, &nullUsedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, time.Time{}, nil, storage.ErrInvalidResetToken
		}
		return 0, time.Time{}, nil, err
	}
	if nullUsedAt.Valid {
		usedAt = &nullUsedAt.Time
	}
	return userID, expiresAt, usedAt, nil
}

func (r *Repo) MarkPasswordResetTokenAsUsed(ctx context.Context, tokenHash string) error {
	const upd = `UPDATE password_reset_tokens SET used_at = $1 WHERE token = $2`
	_, err := r.db.ExecContext(ctx, upd, time.Now().UTC(), tokenHash)
	return err
}

func (r *Repo) InvalidateUserPasswordResetTokens(ctx context.Context, userID int) error {
	const upd = `UPDATE password_reset_tokens SET used_at = $1 WHERE user_id = $2 AND used_at IS NULL`
	_, err := r.db.ExecContext(ctx, upd, time.Now().UTC(), userID)
	return err
}

func (r *Repo) CleanupExpiredPasswordResetTokens(ctx context.Context) (int64, error) {
	const del = `DELETE FROM password_reset_tokens WHERE expires_at < $1 OR used_at IS NOT NULL`
	result, err := r.db.ExecContext(ctx, del, time.Now().UTC().Add(-24*time.Hour))
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

func (r *Repo) ResetPasswordTx(ctx context.Context, tokenHash string, hashedPassword string) (int, error) {
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	const q = `SELECT user_id, expires_at, used_at FROM password_reset_tokens WHERE token = $1 FOR UPDATE`
	var userID int
	var expiresAt time.Time
	var nullUsedAt sql.NullTime

	err = tx.QueryRowContext(ctx, q, tokenHash).Scan(&userID, &expiresAt, &nullUsedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, storage.ErrInvalidResetToken
		}
		return 0, err
	}

	if nullUsedAt.Valid {
		return 0, storage.ErrInvalidResetToken
	}

	if time.Now().After(expiresAt) {
		return 0, storage.ErrInvalidResetToken
	}

	const updPassword = `UPDATE users SET password = $1 WHERE id = $2`
	result, err := tx.ExecContext(ctx, updPassword, hashedPassword, userID)
	if err != nil {
		return 0, err
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return 0, err
	}
	if rowsAffected == 0 {
		return 0, storage.ErrUserNotFound
	}

	const updToken = `UPDATE password_reset_tokens SET used_at = $1 WHERE token = $2`
	if _, err := tx.ExecContext(ctx, updToken, time.Now().UTC(), tokenHash); err != nil {
		return 0, err
	}

	if err := tx.Commit(); err != nil {
		return 0, err
	}

	return userID, nil
}

func (r *Repo) UpdateUserPassword(ctx context.Context, userID int, hashedPassword string) error {
	const upd = `UPDATE users SET password = $1 WHERE id = $2`
	result, err := r.db.ExecContext(ctx, upd, hashedPassword, userID)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return storage.ErrUserNotFound
	}

	return nil
}

func (r *Repo) CreateAuditLog(ctx context.Context, log *core.AuditLog) error {
	const q = `INSERT INTO audit_logs (user_id, action, metadata, ip, user_agent, created_at) VALUES ($1, $2, $3, $4, $5, $6)`
	_, err := r.db.ExecContext(ctx, q, nullInt(&log.UserID), log.Action, nullIfEmpty(log.Metadata), nullIfEmpty(log.IP), nullIfEmpty(log.UserAgent), log.CreatedAt)
	return err
}

// Helpers
func (r *Repo) UpdateUserPermissions(ctx context.Context, userID int, permissions []string) error {
	const q = `UPDATE users SET permissions = $1 WHERE id = $2`
	result, err := r.db.ExecContext(ctx, q, pq.Array(permissions), userID)
	if err != nil {
		return fmt.Errorf("failed to update user permissions: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return storage.ErrUserNotFound
	}

	return nil
}

func nullIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func nullInt(p *int) any {
	if p == nil {
		return nil
	}
	return *p
}
