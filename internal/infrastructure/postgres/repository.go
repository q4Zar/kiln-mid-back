package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/q4ZAr/kiln-mid-back/tezos-delegation-service/internal/domain"
	"github.com/q4ZAr/kiln-mid-back/tezos-delegation-service/pkg/logger"
)

type Repository struct {
	db     *pgxpool.Pool
	logger *logger.Logger
}

func NewRepository(db *pgxpool.Pool, logger *logger.Logger) *Repository {
	return &Repository{
		db:     db,
		logger: logger,
	}
}

func (r *Repository) Save(delegation *domain.Delegation) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if delegation.ID == "" {
		delegation.ID = uuid.New().String()
	}
	if delegation.CreatedAt.IsZero() {
		delegation.CreatedAt = time.Now()
	}

	query := `
		INSERT INTO delegations (id, timestamp, amount, delegator, level, block_hash, operation_hash, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (operation_hash) DO UPDATE SET
			timestamp = EXCLUDED.timestamp,
			amount = EXCLUDED.amount,
			block_hash = EXCLUDED.block_hash,
			delegator = EXCLUDED.delegator,
			level = EXCLUDED.level
	`

	_, err := r.db.Exec(ctx, query,
		delegation.ID,
		delegation.Timestamp,
		delegation.Amount,
		delegation.Delegator,
		delegation.Level,
		delegation.BlockHash,
		delegation.CreatedAt,
	)

	if err != nil {
		r.logger.Errorw("Failed to save delegation", "error", err, "delegation", delegation)
		return fmt.Errorf("failed to save delegation: %w", err)
	}

	return nil
}

func (r *Repository) SaveBatch(delegations []domain.Delegation) error {
	if len(delegations) == 0 {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		// Use a fresh context for rollback to ensure it always works
		tx.Rollback(context.Background())
	}()

	batch := &pgx.Batch{}
	query := `
		INSERT INTO delegations (id, timestamp, amount, delegator, level, block_hash, operation_hash, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (operation_hash) DO UPDATE SET
			timestamp = EXCLUDED.timestamp,
			amount = EXCLUDED.amount,
			block_hash = EXCLUDED.block_hash,
			delegator = EXCLUDED.delegator,
			level = EXCLUDED.level
	`

	for _, delegation := range delegations {
		if delegation.ID == "" {
			delegation.ID = uuid.New().String()
		}
		if delegation.CreatedAt.IsZero() {
			delegation.CreatedAt = time.Now()
		}

		batch.Queue(query,
			delegation.ID,
			delegation.Timestamp,
			delegation.Amount,
			delegation.Delegator,
			delegation.Level,
			delegation.BlockHash,
			delegation.OperationHash,
			delegation.CreatedAt,
		)
	}

	br := tx.SendBatch(ctx, batch)

	successCount := 0
	duplicateCount := 0
	for i := 0; i < batch.Len(); i++ {
		if _, err := br.Exec(); err != nil {
			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) && pgErr.Code == "23505" {
				duplicateCount++
				r.logger.Debugw("Duplicate delegation skipped", "index", i, "code", pgErr.Code, "message", pgErr.Message)
				continue
			}
			br.Close()
			return fmt.Errorf("failed to execute batch item %d: %w", i, err)
		}
		successCount++
	}

	// Close the batch result before committing the transaction
	if err := br.Close(); err != nil {
		return fmt.Errorf("failed to close batch result: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	r.logger.Infow("Saved batch of delegations", "attempted", len(delegations), "saved", successCount, "duplicates", duplicateCount)
	return nil
}

func (r *Repository) FindAll(year *int) ([]domain.Delegation, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var query string
	var args []interface{}

	if year != nil {
		query = `
			SELECT id, timestamp, amount, delegator, level, block_hash, created_at
			FROM delegations
			WHERE EXTRACT(YEAR FROM timestamp) = $1
			ORDER BY timestamp DESC
		`
		args = append(args, *year)
	} else {
		query = `
			SELECT id, timestamp, amount, delegator, level, block_hash, created_at
			FROM delegations
			ORDER BY timestamp DESC
		`
	}

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query delegations: %w", err)
	}
	defer rows.Close()

	var delegations []domain.Delegation
	for rows.Next() {
		var d domain.Delegation
		err := rows.Scan(
			&d.ID,
			&d.Timestamp,
			&d.Amount,
			&d.Delegator,
			&d.Level,
			&d.BlockHash,
			&d.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan delegation: %w", err)
		}
		delegations = append(delegations, d)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	return delegations, nil
}

func (r *Repository) GetLastIndexedLevel() (int64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var lastLevel sql.NullInt64
	query := `
		SELECT MAX(CAST(level AS BIGINT))
		FROM delegations
	`

	err := r.db.QueryRow(ctx, query).Scan(&lastLevel)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, nil
		}
		return 0, fmt.Errorf("failed to get last indexed level: %w", err)
	}

	if !lastLevel.Valid {
		return 0, nil
	}

	return lastLevel.Int64, nil
}

func (r *Repository) Exists(delegator string, level string) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var exists bool
	query := `
		SELECT EXISTS(
			SELECT 1 FROM delegations 
			WHERE delegator = $1 AND level = $2
		)
	`

	err := r.db.QueryRow(ctx, query, delegator, level).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("failed to check if delegation exists: %w", err)
	}

	return exists, nil
}

func (r *Repository) UpdateIndexingMetadata(level int64, timestamp time.Time) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	query := `
		UPDATE indexing_metadata
		SET last_indexed_level = $1,
		    last_indexed_timestamp = $2,
		    updated_at = NOW()
		WHERE id = 1
	`

	_, err := r.db.Exec(ctx, query, level, timestamp)
	if err != nil {
		return fmt.Errorf("failed to update indexing metadata: %w", err)
	}

	return nil
}

func (r *Repository) GetIndexingMetadata() (int64, *time.Time, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var level int64
	var timestamp sql.NullTime

	query := `
		SELECT last_indexed_level, last_indexed_timestamp
		FROM indexing_metadata
		WHERE id = 1
	`

	err := r.db.QueryRow(ctx, query).Scan(&level, &timestamp)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, nil, nil
		}
		return 0, nil, fmt.Errorf("failed to get indexing metadata: %w", err)
	}

	if timestamp.Valid {
		return level, &timestamp.Time, nil
	}

	return level, nil, nil
}

func (r *Repository) GetDelegationsByTimeRange(start, end time.Time) ([]domain.Delegation, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	query := `
		SELECT id, timestamp, amount, delegator, level, block_hash, created_at
		FROM delegations
		WHERE timestamp >= $1 AND timestamp <= $2
		ORDER BY timestamp DESC
	`

	rows, err := r.db.Query(ctx, query, start, end)
	if err != nil {
		return nil, fmt.Errorf("failed to query delegations by time range: %w", err)
	}
	defer rows.Close()

	var delegations []domain.Delegation
	for rows.Next() {
		var d domain.Delegation
		err := rows.Scan(
			&d.ID,
			&d.Timestamp,
			&d.Amount,
			&d.Delegator,
			&d.Level,
			&d.BlockHash,
			&d.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan delegation: %w", err)
		}
		delegations = append(delegations, d)
	}

	return delegations, nil
}

func (r *Repository) GetStats() (map[string]interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	stats := make(map[string]interface{})

	var totalCount int64
	err := r.db.QueryRow(ctx, "SELECT COUNT(*) FROM delegations").Scan(&totalCount)
	if err != nil {
		return nil, fmt.Errorf("failed to get total count: %w", err)
	}
	stats["total_delegations"] = totalCount

	var totalAmount sql.NullString
	err = r.db.QueryRow(ctx, `
		SELECT SUM(CAST(amount AS NUMERIC))::TEXT 
		FROM delegations
	`).Scan(&totalAmount)
	if err != nil {
		return nil, fmt.Errorf("failed to get total amount: %w", err)
	}
	if totalAmount.Valid {
		stats["total_amount"] = totalAmount.String
	} else {
		stats["total_amount"] = "0"
	}

	var uniqueDelegators int64
	err = r.db.QueryRow(ctx, "SELECT COUNT(DISTINCT delegator) FROM delegations").Scan(&uniqueDelegators)
	if err != nil {
		return nil, fmt.Errorf("failed to get unique delegators: %w", err)
	}
	stats["unique_delegators"] = uniqueDelegators

	var lastLevel sql.NullString
	err = r.db.QueryRow(ctx, `
		SELECT level FROM delegations 
		ORDER BY CAST(level AS BIGINT) DESC 
		LIMIT 1
	`).Scan(&lastLevel)
	if err != nil && err != pgx.ErrNoRows {
		return nil, fmt.Errorf("failed to get last level: %w", err)
	}
	if lastLevel.Valid {
		if l, err := strconv.ParseInt(lastLevel.String, 10, 64); err == nil {
			stats["last_indexed_level"] = l
		}
	}

	return stats, nil
}
