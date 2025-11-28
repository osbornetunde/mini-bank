package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"mini-bank/internal/core"
	"mini-bank/internal/storage"

	"golang.org/x/crypto/bcrypt"
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
	const q = `INSERT INTO accounts (user_id, balance) VALUES ($1, $2) RETURNING id, user_id, balance, created_at`
	row := r.db.QueryRowContext(ctx, q, userID, balance)
	return scanAccount(row)
}

// Helper to scan account
func scanAccount(row scanner) (*core.Account, error) {
	var a core.Account
	if err := row.Scan(&a.ID, &a.UserID, &a.Balance, &a.CreatedAt); err != nil {
		return nil, err
	}
	return &a, nil
}

func scanTransaction(row scanner) (*core.Transaction, error) {
	var t core.Transaction
	if err := row.Scan(&t.ID, &t.AccountID, &t.Type, &t.Amount, &t.Reference, &t.FromAccountID, &t.ToAccountID, &t.Timestamp); err != nil {
		return nil, err
	}
	return &t, nil
}

func scanUser(row scanner) (*core.User, error) {
	var u core.User
	if err := row.Scan(&u.ID, &u.FirstName, &u.LastName, &u.Email, &u.Balance); err != nil {
		return nil, err
	}
	return &u, nil
}

type scanner interface {
	Scan(dest ...any) error
}

// GetAccount retrieves an account by id
func (r *Repo) GetAccount(ctx context.Context, id int) (*core.Account, error) {
	const q = `SELECT id, user_id, balance, created_at FROM accounts WHERE id = $1`
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
	const q = `SELECT id, user_id, balance, created_at FROM accounts ORDER BY id`
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
	const upd = `UPDATE accounts SET balance = balance + $1 WHERE id = $2 RETURNING id, user_id, balance, created_at`
	var acc core.Account
	if err := tx.QueryRowContext(ctx, upd, amount, accountID).Scan(&acc.ID, &acc.UserID, &acc.Balance, &acc.CreatedAt); err != nil {
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
func (r *Repo) Withdraw(ctx context.Context, accountID int, amount int64, reference string) (*core.Account, error) {
	if amount <= 0 {
		return nil, errors.New("amount must be positive")
	}

	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	// Attempt to debit if sufficient funds exist; RETURNING gives new account details
	const debit = `UPDATE accounts SET balance = balance - $1 WHERE id = $2 AND balance >= $1 RETURNING id, user_id, balance, created_at`
	var acc core.Account
	if err := tx.QueryRowContext(ctx, debit, amount, accountID).Scan(&acc.ID, &acc.UserID, &acc.Balance, &acc.CreatedAt); err != nil {
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

	// Insert transaction record
	const ins = `INSERT INTO transactions (account_id, type, amount, reference, created_at) VALUES ($1, $2, $3, $4, $5)`
	if _, err := tx.ExecContext(ctx, ins, accountID, "withdraw", amount, nullIfEmpty(reference), time.Now().UTC()); err != nil {
		return nil, err
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
	const q = `SELECT id, account_id, type, amount, reference, from_account_id, to_account_id, created_at FROM transactions WHERE account_id = $1 ORDER BY created_at DESC`
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
		if err := rows.Scan(&t.ID, &t.AccountID, &t.Type, &t.Amount, &ref, &from, &to, &t.Timestamp); err != nil {
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
		res = append(res, &t)
	}
	return res, rows.Err()
}

// UpdateBalance updates an account's balance.
func (r *Repo) UpdateBalance(ctx context.Context, id int, newBalance int64) error {
	const q = `UPDATE accounts SET balance = $1 WHERE id = $2`
	_, err := r.db.ExecContext(ctx, q, newBalance, id)
	return err
}

// Transfer performs a transactional transfer between two accounts.
func (r *Repo) Transfer(ctx context.Context, fromID, toID int, amount int64, reference string) (*core.Account, *core.Account, error) {
	if amount <= 0 {
		return nil, nil, errors.New("amount must be positive")
	}

	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return nil, nil, err
	}
	defer tx.Rollback()

	// Withdraw from sender
	const debit = `UPDATE accounts SET balance = balance - $1 WHERE id = $2 AND balance >= $1 RETURNING id, user_id, balance, created_at`
	var fromAcc core.Account
	if err := tx.QueryRowContext(ctx, debit, amount, fromID).Scan(&fromAcc.ID, &fromAcc.UserID, &fromAcc.Balance, &fromAcc.CreatedAt); err != nil {
		if err == sql.ErrNoRows {
			// This could mean insufficient funds or the account doesn't exist.
			// A more robust implementation could check for existence first.
			return nil, nil, storage.ErrInsufficientFunds
		}
		return nil, nil, err
	}

	// Deposit to receiver
	const credit = `UPDATE accounts SET balance = balance + $1 WHERE id = $2 RETURNING id, user_id, balance, created_at`
	var toAcc core.Account
	if err := tx.QueryRowContext(ctx, credit, amount, toID).Scan(&toAcc.ID, &toAcc.UserID, &toAcc.Balance, &toAcc.CreatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil, storage.ErrAccountNotFound
		}
		return nil, nil, err
	}

	// Record transaction for sender
	const insFrom = `INSERT INTO transactions (account_id, type, amount, to_account_id, reference, created_at) VALUES ($1, 'transfer', $2, $3, $4, $5)`
	if _, err := tx.ExecContext(ctx, insFrom, fromID, amount, toID, nullIfEmpty(reference), time.Now().UTC()); err != nil {
		return nil, nil, err
	}

	// Record transaction for receiver
	const insTo = `INSERT INTO transactions (account_id, type, amount, from_account_id, reference, created_at) VALUES ($1, 'transfer', $2, $3, $4, $5)`
	if _, err := tx.ExecContext(ctx, insTo, toID, amount, fromID, nullIfEmpty(reference), time.Now().UTC()); err != nil {
		return nil, nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, nil, err
	}

	return &fromAcc, &toAcc, nil
}

// Payment performs a deposit or withdrawal and returns the updated account.
func (r *Repo) Payment(ctx context.Context, accountID int, amount int64, paymentType storage.PaymentType, reference string) (*core.Account, error) {
	switch paymentType {
	case storage.Deposit:
		return r.Deposit(ctx, accountID, amount, reference)
	case storage.Withdraw:
		return r.Withdraw(ctx, accountID, amount, reference)
	default:
		return nil, fmt.Errorf("unknown payment type: %s", paymentType)
	}
}

func (r *Repo) GetTransaction(ctx context.Context, ref string) (*core.Transaction, error) {
	const q = `SELECT id, account_id, type, amount, reference, from_account_id, to_account_id, created_at FROM transactions WHERE reference = $1`

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
	const ins = `INSERT INTO users (first_name, last_name, email, password) VALUES ($1, $2, $3, $4) RETURNING id`

	var id int
	hashedPassword, err := hashPassword(password)
	if err != nil {
		return nil, err
	}
	row := r.db.QueryRowContext(ctx, ins, firstName, lastName, email, hashedPassword)
	if err := row.Scan(&id); err != nil {
		return nil, err
	}

	if _, err := r.CreateAccount(ctx, id, 0); err != nil {
		return nil, err
	}

	return &core.User{ID: id, FirstName: firstName, LastName: lastName, Email: email}, nil
}

func (r *Repo) GetUsers(ctx context.Context) ([]*core.User, error) {
	q := `SELECT id, email, first_name, last_name FROM users`
	rows, err := r.db.QueryContext(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var users []*core.User

	for rows.Next() {
		var user core.User
		if err := rows.Scan(&user.ID, &user.Email, &user.FirstName, &user.LastName); err != nil {
			return nil, err
		}
		users = append(users, &user)
	}
	return users, nil
}

func (r *Repo) GetUser(ctx context.Context, userId int) (*core.User, error) {
	q := `SELECT u.id, u.first_name, u.last_name, u.email, a.balance FROM users u INNER JOIN accounts a ON u.id = a.user_id WHERE u.id = $1`
	row := r.db.QueryRowContext(ctx, q, userId)
	user, err := scanUser(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, storage.ErrUserNotFound
		}
		return nil, err
	}
	return user, nil
}

func (r *Repo) UpdateUser(ctx context.Context, id int, firstName, lastName, email string) (*core.User, error) {
	var user *core.User
	q := `UPDATE users u SET first_name = $2, last_name = $3, email = $4 FROM accounts a WHERE u.id = $1 AND a.user_id = u.id RETURNING u.id, u.first_name, u.last_name, u.email, a.balance`
	row := r.db.QueryRowContext(ctx, q, id, firstName, lastName, email)
	user, err := scanUser(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, storage.ErrUserNotFound
		}
		return nil, err
	}
	return user, nil
}

// Helpers
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

func hashPassword(password string) (string, error) {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hashedPassword), nil
}

func verifyPassword(hashedPassword string, password string) error {
	return bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(password))
}
