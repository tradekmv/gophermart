// Package storage provides PostgreSQL implementation of the Storage interface.
package storage

import (
	"context"
	"embed"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/rs/zerolog"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

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

// PostgresStorage implements Storage interface using PostgreSQL via pgxpool.
type PostgresStorage struct {
	pool   *pgxpool.Pool
	logger *zerolog.Logger
}

// NewPostgresStorage creates a new PostgreSQL storage instance, applies migrations.
// logger — опциональный, может быть nil. Если передан — все ошибки логируются
// с дополнительным контекстом (rollback, scan и т.д.).
func NewPostgresStorage(ctx context.Context, databaseURI string, logger *zerolog.Logger) (*PostgresStorage, error) {
	cfg, err := pgxpool.ParseConfig(databaseURI)
	if err != nil {
		return nil, fmt.Errorf("parse pgxpool config: %w", err)
	}

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("pgxpool new: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}

	if err := runMigrations(ctx, pool); err != nil {
		pool.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}

	return &PostgresStorage{pool: pool, logger: logger}, nil
}

// runMigrations applies embedded goose migrations on the given pool.
// Использует pgxpool через stdlib-обёртку, так как goose v3 работает с *sql.DB.
// sqlDB закрывается **перед** возвратом — её Close закроет и пул,
// поэтому PostgresStorage хранит только pool.
//
// Идемпотентность: goose хранит версии применённых миграций в таблице
// `goose_db_version`. Повторный вызов UpContext возвращает ошибку только
// если предыдущий запуск был прерван посреди миграции (и тогда goose
// сам поднимет panic: "missing migration"). См. https://pressly.github.io/goose/
// — поэтому DROP SCHEMA public нужен только если ранее накатывались миграции
// НЕ через goose (например, ручной CREATE TABLE в gophermarttest).
func runMigrations(ctx context.Context, pool *pgxpool.Pool) error {
	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("goose dialect: %w", err)
	}
	goose.SetBaseFS(migrationsFS)

	sqlDB := stdlib.OpenDBFromPool(pool)
	if sqlDB == nil {
		return fmt.Errorf("stdlib open: returned nil *sql.DB")
	}
	// Закрываем sqlDB ДО возврата — иначе goose.Up держит соединение.
	// ВАЖНО: sqlDB.Close() закроет и pool. Поэтому мы НЕ сохраняем sqlDB
	// в PostgresStorage, и в Close() вызываем только pool.Close().
	defer sqlDB.Close()

	if err := goose.UpContext(ctx, sqlDB, "migrations"); err != nil {
		return fmt.Errorf("goose up: %w", err)
	}
	return nil
}

// Close closes the database connection pool.
func (s *PostgresStorage) Close() error {
	s.pool.Close()
	return nil
}

// CreateUserWithBalance creates a new user and their initial balance in a transaction.
func (s *PostgresStorage) CreateUserWithBalance(ctx context.Context, login, passwordHash string) (string, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return "", fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Insert user
	var userID string
	err = tx.QueryRow(ctx,
		`INSERT INTO users (login, password_hash) VALUES ($1, $2) RETURNING id`,
		login, passwordHash,
	).Scan(&userID)
	if err != nil {
		if isUniqueViolation(err, "users_login_key") {
			return "", ErrUserExists
		}
		return "", fmt.Errorf("insert user: %w", err)
	}

	// Insert balance
	_, err = tx.Exec(ctx,
		`INSERT INTO balances (user_id, current, withdrawn) VALUES ($1, 0, 0)`,
		userID,
	)
	if err != nil {
		return "", fmt.Errorf("insert balance: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return "", fmt.Errorf("commit tx: %w", err)
	}

	return userID, nil
}

// isUniqueViolation возвращает true, если err — pgx-ошибка PostgreSQL
// с кодом 23505 (unique_violation) и именем ограничения == constraintName.
// Без проверки ConstraintName можно ошибочно обработать 23505 от другого
// уникального ограничения (например, если в будущем в `users` добавится
// `email UNIQUE`).
func isUniqueViolation(err error, constraintName string) bool {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "23505" {
		return false
	}
	return pgErr.ConstraintName == constraintName
}

// GetUserByLogin retrieves a user by their login.
func (s *PostgresStorage) GetUserByLogin(ctx context.Context, login string) (User, error) {
	var user User
	err := s.pool.QueryRow(ctx,
		`SELECT id, login, password_hash, created_at FROM users WHERE login = $1`,
		login,
	).Scan(&user.ID, &user.Login, &user.PasswordHash, &user.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return User{}, ErrNotFound
		}
		return User{}, fmt.Errorf("query user: %w", err)
	}
	return user, nil
}

// SaveOrder saves a new order for a user.
func (s *PostgresStorage) SaveOrder(ctx context.Context, userID, orderNumber string) (Order, error) {
	var order Order
	var accrual *float64

	err := s.pool.QueryRow(ctx,
		`INSERT INTO orders (user_id, number, status, uploaded_at)
		 VALUES ($1, $2, 'NEW', NOW())
		 ON CONFLICT (number) DO NOTHING
		 RETURNING id, user_id, number, status, accrual, uploaded_at`,
		userID, orderNumber,
	).Scan(&order.ID, &order.UserID, &order.Number, &order.Status, &accrual, &order.UploadedAt)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Order{}, ErrOrderExists
		}
		return Order{}, fmt.Errorf("insert order: %w", err)
	}

	order.Accrual = accrual

	return order, nil
}

// GetOrderByNumber retrieves an order by its number.
func (s *PostgresStorage) GetOrderByNumber(ctx context.Context, orderNumber string) (Order, error) {
	var order Order
	var accrual *float64

	err := s.pool.QueryRow(ctx,
		`SELECT id, user_id, number, status, accrual, uploaded_at FROM orders WHERE number = $1`,
		orderNumber,
	).Scan(&order.ID, &order.UserID, &order.Number, &order.Status, &accrual, &order.UploadedAt)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Order{}, ErrNotFound
		}
		return Order{}, fmt.Errorf("query order: %w", err)
	}

	order.Accrual = accrual

	return order, nil
}

// GetOrdersByUserID retrieves all orders for a user, sorted by uploaded_at DESC.
func (s *PostgresStorage) GetOrdersByUserID(ctx context.Context, userID string) ([]Order, error) {
	rows, err := s.pool.Query(ctx,
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
		var accrual *float64

		if err := rows.Scan(&order.ID, &order.UserID, &order.Number, &order.Status, &accrual, &order.UploadedAt); err != nil {
			return nil, fmt.Errorf("scan order: %w", err)
		}
		order.Accrual = accrual
		orders = append(orders, order)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %w", err)
	}

	return orders, nil
}

// GetPendingOrders retrieves orders with status NEW or PROCESSING.
func (s *PostgresStorage) GetPendingOrders(ctx context.Context) ([]Order, error) {
	rows, err := s.pool.Query(ctx,
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
		var accrual *float64

		if err := rows.Scan(&order.ID, &order.UserID, &order.Number, &order.Status, &accrual, &order.UploadedAt); err != nil {
			return nil, fmt.Errorf("scan order: %w", err)
		}
		order.Accrual = accrual
		orders = append(orders, order)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %w", err)
	}

	return orders, nil
}

// UpdateOrderByNumber updates an order's status and accrual.
func (s *PostgresStorage) UpdateOrderByNumber(ctx context.Context, number, status string, accrual *float64) error {
	if accrual != nil {
		_, err := s.pool.Exec(ctx,
			`UPDATE orders SET status = $1, accrual = $2 WHERE number = $3`,
			status, *accrual, number,
		)
		if err != nil {
			return fmt.Errorf("update order with accrual: %w", err)
		}
	} else {
		_, err := s.pool.Exec(ctx,
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
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Update balance
	_, err = tx.Exec(ctx,
		`UPDATE balances SET current = current + $1 WHERE user_id = $2`,
		accrual, userID,
	)
	if err != nil {
		return fmt.Errorf("update balance: %w", err)
	}

	// Update order status and accrual
	_, err = tx.Exec(ctx,
		`UPDATE orders SET status = 'PROCESSED', accrual = $1 WHERE number = $2`,
		accrual, orderNumber,
	)
	if err != nil {
		return fmt.Errorf("update order: %w", err)
	}

	return tx.Commit(ctx)
}

// GetBalance retrieves user's current and withdrawn balance.
func (s *PostgresStorage) GetBalance(ctx context.Context, userID string) (float64, float64, error) {
	var current, withdrawn float64
	err := s.pool.QueryRow(ctx,
		`SELECT current, withdrawn FROM balances WHERE user_id = $1`,
		userID,
	).Scan(&current, &withdrawn)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, 0, ErrNotFound
		}
		return 0, 0, fmt.Errorf("query balance: %w", err)
	}

	return current, withdrawn, nil
}

// Withdraw processes a withdrawal request atomically.
func (s *PostgresStorage) Withdraw(ctx context.Context, userID, order string, sum float64) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Check and lock balance
	var current float64
	err = tx.QueryRow(ctx,
		`SELECT current FROM balances WHERE user_id = $1 FOR UPDATE`,
		userID,
	).Scan(&current)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return fmt.Errorf("lock balance: %w", err)
	}

	// Check sufficient funds
	if current < sum {
		return ErrInsufficientFunds
	}

	// Update balance
	_, err = tx.Exec(ctx,
		`UPDATE balances SET current = current - $1, withdrawn = withdrawn + $1 WHERE user_id = $2`,
		sum, userID,
	)
	if err != nil {
		return fmt.Errorf("update balance: %w", err)
	}

	// Insert withdrawal record
	_, err = tx.Exec(ctx,
		`INSERT INTO withdrawals (user_id, "order", sum) VALUES ($1, $2, $3)`,
		userID, order, sum,
	)
	if err != nil {
		return fmt.Errorf("insert withdrawal: %w", err)
	}

	return tx.Commit(ctx)
}

// GetWithdrawals retrieves all withdrawals for a user.
func (s *PostgresStorage) GetWithdrawals(ctx context.Context, userID string) ([]Withdrawal, error) {
	rows, err := s.pool.Query(ctx,
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

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %w", err)
	}

	return withdrawals, nil
}
