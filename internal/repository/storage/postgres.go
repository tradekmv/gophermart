// Package storage provides PostgreSQL implementation of the Storage interface.
package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	_ "github.com/jackc/pgx/v5/stdlib"
)

var (
	// ErrNotFound is returned when a requested resource is not found.
	ErrNotFound = errors.New("not found")
	// ErrUserExists is returned when trying to create a user with existing login.
	ErrUserExists = errors.New("user already exists")
	// ErrOrderExists is returned when order is already uploaded.
	ErrOrderExists = errors.New("order already exists")
	// ErrInsufficientFunds is returned when user has insufficient balance.
	ErrInsufficientFunds = errors.New("insufficient funds")
)

// PostgresStorage implements Storage interface using PostgreSQL.
type PostgresStorage struct {
	db *sql.DB
}

// NewPostgresStorage creates a new PostgreSQL storage instance.
func NewPostgresStorage(databaseURI string) (*PostgresStorage, error) {
	db, err := sql.Open("pgx", databaseURI)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("ping database: %w", err)
	}

	s := &PostgresStorage{db: db}

	if err := s.migrate(); err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}

	return s, nil
}

// migrate creates database tables if they don't exist.
func (s *PostgresStorage) migrate() error {
	migrations := []string{
		`CREATE TABLE IF NOT EXISTS users (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			login VARCHAR(255) UNIQUE NOT NULL,
			password_hash VARCHAR(255) NOT NULL,
			created_at TIMESTAMP DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS orders (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			user_id UUID REFERENCES users(id),
			number VARCHAR(255) NOT NULL,
			status VARCHAR(20) DEFAULT 'NEW',
			accrual DECIMAL(10,2),
			uploaded_at TIMESTAMP DEFAULT NOW(),
			CONSTRAINT orders_number_unique UNIQUE (number)
		)`,
		`CREATE TABLE IF NOT EXISTS balances (
			user_id UUID PRIMARY KEY REFERENCES users(id),
			current DECIMAL(10,2) DEFAULT 0,
			withdrawn DECIMAL(10,2) DEFAULT 0
		)`,
		`CREATE TABLE IF NOT EXISTS withdrawals (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			user_id UUID REFERENCES users(id),
			"order" VARCHAR(255) NOT NULL,
			sum DECIMAL(10,2) NOT NULL,
			processed_at TIMESTAMP DEFAULT NOW()
		)`,
	}

	for _, m := range migrations {
		if _, err := s.db.Exec(m); err != nil {
			return fmt.Errorf("exec migration: %w", err)
		}
	}

	// Create indexes
	indexes := []string{
		`CREATE INDEX IF NOT EXISTS idx_orders_user_id ON orders(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_orders_status ON orders(status)`,
		`CREATE INDEX IF NOT EXISTS idx_orders_status_user ON orders(status, user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_withdrawals_user_id ON withdrawals(user_id)`,
	}

	for _, idx := range indexes {
		if _, err := s.db.Exec(idx); err != nil {
			return fmt.Errorf("create index: %w", err)
		}
	}

	return nil
}

// Close closes the database connection.
func (s *PostgresStorage) Close() error {
	return s.db.Close()
}

// CreateUserWithBalance creates a new user and their initial balance in a transaction.
func (s *PostgresStorage) CreateUserWithBalance(ctx context.Context, login, passwordHash string) (string, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return "", fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	// Insert user
	var userID string
	err = tx.QueryRowContext(ctx,
		`INSERT INTO users (login, password_hash) VALUES ($1, $2) RETURNING id`,
		login, passwordHash,
	).Scan(&userID)
	if err != nil {
		if strings.Contains(err.Error(), "unique constraint") || strings.Contains(err.Error(), "duplicate key") {
			return "", ErrUserExists
		}
		return "", fmt.Errorf("insert user: %w", err)
	}

	// Insert balance
	_, err = tx.ExecContext(ctx,
		`INSERT INTO balances (user_id, current, withdrawn) VALUES ($1, 0, 0)`,
		userID,
	)
	if err != nil {
		return "", fmt.Errorf("insert balance: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return "", fmt.Errorf("commit tx: %w", err)
	}

	return userID, nil
}

// GetUserByLogin retrieves a user by their login.
func (s *PostgresStorage) GetUserByLogin(ctx context.Context, login string) (User, error) {
	var user User
	err := s.db.QueryRowContext(ctx,
		`SELECT id, login, password_hash, created_at FROM users WHERE login = $1`,
		login,
	).Scan(&user.ID, &user.Login, &user.PasswordHash, &user.CreatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return User{}, ErrNotFound
		}
		return User{}, fmt.Errorf("query user: %w", err)
	}
	return user, nil
}

// SaveOrder saves a new order for a user.
func (s *PostgresStorage) SaveOrder(ctx context.Context, userID, orderNumber string) (Order, error) {
	var order Order
	var accrual sql.NullFloat64

	err := s.db.QueryRowContext(ctx,
		`INSERT INTO orders (user_id, number, status, uploaded_at)
		 VALUES ($1, $2, 'NEW', NOW())
		 ON CONFLICT (number) DO NOTHING
		 RETURNING id, user_id, number, status, accrual, uploaded_at`,
		userID, orderNumber,
	).Scan(&order.ID, &order.UserID, &order.Number, &order.Status, &accrual, &order.UploadedAt)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Order{}, ErrOrderExists
		}
		return Order{}, fmt.Errorf("insert order: %w", err)
	}

	if accrual.Valid {
		order.Accrual = &accrual.Float64
	}

	return order, nil
}

// GetOrderByNumber retrieves an order by its number.
func (s *PostgresStorage) GetOrderByNumber(ctx context.Context, orderNumber string) (Order, error) {
	var order Order
	var accrual sql.NullFloat64

	err := s.db.QueryRowContext(ctx,
		`SELECT id, user_id, number, status, accrual, uploaded_at FROM orders WHERE number = $1`,
		orderNumber,
	).Scan(&order.ID, &order.UserID, &order.Number, &order.Status, &accrual, &order.UploadedAt)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Order{}, ErrNotFound
		}
		return Order{}, fmt.Errorf("query order: %w", err)
	}

	if accrual.Valid {
		order.Accrual = &accrual.Float64
	}

	return order, nil
}

// GetOrdersByUserID retrieves all orders for a user, sorted by uploaded_at DESC.
func (s *PostgresStorage) GetOrdersByUserID(ctx context.Context, userID string) ([]Order, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, user_id, number, status, accrual, uploaded_at 
		 FROM orders WHERE user_id = $1 ORDER BY uploaded_at DESC`,
		userID,
	)
	if err != nil {
		return nil, fmt.Errorf("query orders: %w", err)
	}
	defer rows.Close()

	var orders []Order
	for rows.Next() {
		var order Order
		var accrual sql.NullFloat64

		if err := rows.Scan(&order.ID, &order.UserID, &order.Number, &order.Status, &accrual, &order.UploadedAt); err != nil {
			return nil, fmt.Errorf("scan order: %w", err)
		}

		if accrual.Valid {
			order.Accrual = &accrual.Float64
		}
		orders = append(orders, order)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %w", err)
	}

	return orders, nil
}

// GetPendingOrders retrieves orders with status NEW or PROCESSING.
func (s *PostgresStorage) GetPendingOrders(ctx context.Context) ([]Order, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, user_id, number, status, accrual, uploaded_at 
		 FROM orders WHERE status IN ('NEW', 'PROCESSING')`,
	)
	if err != nil {
		return nil, fmt.Errorf("query pending orders: %w", err)
	}
	defer rows.Close()

	var orders []Order
	for rows.Next() {
		var order Order
		var accrual sql.NullFloat64

		if err := rows.Scan(&order.ID, &order.UserID, &order.Number, &order.Status, &accrual, &order.UploadedAt); err != nil {
			return nil, fmt.Errorf("scan order: %w", err)
		}

		if accrual.Valid {
			order.Accrual = &accrual.Float64
		}
		orders = append(orders, order)
	}

	return orders, nil
}

// UpdateOrderByNumber updates an order's status and accrual.
func (s *PostgresStorage) UpdateOrderByNumber(ctx context.Context, number, status string, accrual *float64) error {
	if accrual != nil {
		_, err := s.db.ExecContext(ctx,
			`UPDATE orders SET status = $1, accrual = $2 WHERE number = $3`,
			status, *accrual, number,
		)
		if err != nil {
			return fmt.Errorf("update order with accrual: %w", err)
		}
	} else {
		_, err := s.db.ExecContext(ctx,
			`UPDATE orders SET status = $1 WHERE number = $2`,
			status, number,
		)
		if err != nil {
			return fmt.Errorf("update order: %w", err)
		}
	}
	return nil
}

// AddAccrual atomically adds accrual to user's balance and updates order status.
func (s *PostgresStorage) AddAccrual(ctx context.Context, userID, orderNumber string, accrual float64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	// Update balance
	_, err = tx.ExecContext(ctx,
		`UPDATE balances SET current = current + $1 WHERE user_id = $2`,
		accrual, userID,
	)
	if err != nil {
		return fmt.Errorf("update balance: %w", err)
	}

	// Update order status and accrual
	_, err = tx.ExecContext(ctx,
		`UPDATE orders SET status = 'PROCESSED', accrual = $1 WHERE number = $2`,
		accrual, orderNumber,
	)
	if err != nil {
		return fmt.Errorf("update order: %w", err)
	}

	return tx.Commit()
}

// GetBalance retrieves user's current and withdrawn balance.
func (s *PostgresStorage) GetBalance(ctx context.Context, userID string) (float64, float64, error) {
	var current, withdrawn float64
	err := s.db.QueryRowContext(ctx,
		`SELECT current, withdrawn FROM balances WHERE user_id = $1`,
		userID,
	).Scan(&current, &withdrawn)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, 0, ErrNotFound
		}
		return 0, 0, fmt.Errorf("query balance: %w", err)
	}

	return current, withdrawn, nil
}

// Withdraw processes a withdrawal request atomically.
func (s *PostgresStorage) Withdraw(ctx context.Context, userID, order string, sum float64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	// Check and lock balance
	var current float64
	err = tx.QueryRowContext(ctx,
		`SELECT current FROM balances WHERE user_id = $1 FOR UPDATE`,
		userID,
	).Scan(&current)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		return fmt.Errorf("lock balance: %w", err)
	}

	// Check sufficient funds
	if current < sum {
		return ErrInsufficientFunds
	}

	// Update balance
	_, err = tx.ExecContext(ctx,
		`UPDATE balances SET current = current - $1, withdrawn = withdrawn + $1 WHERE user_id = $2`,
		sum, userID,
	)
	if err != nil {
		return fmt.Errorf("update balance: %w", err)
	}

	// Insert withdrawal record
	_, err = tx.ExecContext(ctx,
		`INSERT INTO withdrawals (user_id, "order", sum) VALUES ($1, $2::VARCHAR, $3)`,
		userID, order, sum,
	)
	if err != nil {
		return fmt.Errorf("insert withdrawal: %w", err)
	}

	return tx.Commit()
}

// GetWithdrawals retrieves all withdrawals for a user.
func (s *PostgresStorage) GetWithdrawals(ctx context.Context, userID string) ([]Withdrawal, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, user_id, "order", sum, processed_at 
		 FROM withdrawals WHERE user_id = $1 ORDER BY processed_at DESC`,
		userID,
	)
	if err != nil {
		return nil, fmt.Errorf("query withdrawals: %w", err)
	}
	defer rows.Close()

	var withdrawals []Withdrawal
	for rows.Next() {
		var w Withdrawal
		if err := rows.Scan(&w.ID, &w.UserID, &w.Order, &w.Sum, &w.ProcessedAt); err != nil {
			return nil, fmt.Errorf("scan withdrawal: %w", err)
		}
		withdrawals = append(withdrawals, w)
	}

	return withdrawals, nil
}
