package storage

import (
	"context"
	"database/sql"
	"fmt"
	"math/big"
	"time"

	"insolventbydesign/internal/model"

	_ "github.com/lib/pq"
)

// PostgresStore provides TimescaleDB-optimized storage for censorship data.
type PostgresStore struct {
	db *sql.DB
}

// Config contains database connection parameters.
type Config struct {
	Host     string
	Port     int
	User     string
	Password string
	Database string
	SSLMode  string
}

// NewPostgresStore creates a new database connection with connection pooling.
func NewPostgresStore(config Config) (*PostgresStore, error) {
	connStr := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		config.Host, config.Port, config.User, config.Password, config.Database, config.SSLMode)

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Connection pool configuration for high throughput
	db.SetMaxOpenConns(100)
	db.SetMaxIdleConns(25)
	db.SetConnMaxLifetime(5 * time.Minute)

	// Verify connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return &PostgresStore{db: db}, nil
}

// InitSchema creates the database schema with TimescaleDB hypertable.
func (s *PostgresStore) InitSchema(ctx context.Context) error {
	schema := `
	-- Enable TimescaleDB extension
	CREATE EXTENSION IF NOT EXISTS timescaledb;
	
	-- Slot bribes table (time-series data)
	CREATE TABLE IF NOT EXISTS slot_bribes (
		slot_number BIGINT NOT NULL,
		slot_time TIMESTAMPTZ NOT NULL,
		value_wei NUMERIC(78, 0) NOT NULL,  -- Supports up to 2^256
		value_eth DOUBLE PRECISION NOT NULL,
		builder_pubkey TEXT NOT NULL,
		block_hash TEXT NOT NULL,
		relay_url TEXT NOT NULL,
		fetched_at TIMESTAMPTZ DEFAULT NOW(),
		PRIMARY KEY (slot_time, slot_number)
	);
	
	-- Convert to hypertable for time-series optimization
	SELECT create_hypertable('slot_bribes', 'slot_time', if_not_exists => TRUE);
	
	-- Indexes for common queries
	CREATE INDEX IF NOT EXISTS idx_slot_bribes_slot ON slot_bribes (slot_number);
	CREATE INDEX IF NOT EXISTS idx_slot_bribes_builder ON slot_bribes (builder_pubkey);
	CREATE INDEX IF NOT EXISTS idx_slot_bribes_value ON slot_bribes (value_eth DESC);
	
	-- Builder statistics materialized view (auto-refreshing)
	CREATE MATERIALIZED VIEW IF NOT EXISTS builder_stats AS
	SELECT 
		builder_pubkey,
		COUNT(*) as block_count,
		SUM(value_eth) as total_value_eth,
		AVG(value_eth) as avg_value_eth,
		MAX(value_eth) as max_value_eth,
		MIN(value_eth) as min_value_eth,
		STDDEV(value_eth) as stddev_value_eth
	FROM slot_bribes
	GROUP BY builder_pubkey
	ORDER BY block_count DESC;
	
	CREATE UNIQUE INDEX IF NOT EXISTS idx_builder_stats_pubkey ON builder_stats (builder_pubkey);
	
	-- Censorship cost analysis table
	CREATE TABLE IF NOT EXISTS censorship_analysis (
		id SERIAL PRIMARY KEY,
		start_slot BIGINT NOT NULL,
		end_slot BIGINT NOT NULL,
		duration_slots INT NOT NULL,
		total_cost_wei NUMERIC(78, 0) NOT NULL,
		total_cost_eth DOUBLE PRECISION NOT NULL,
		total_cost_usd DOUBLE PRECISION,
		builder_concentration DOUBLE PRECISION NOT NULL,
		top_k_builders INT NOT NULL,
		effective_cost_eth DOUBLE PRECISION NOT NULL,
		breakeven_tvl_usd DOUBLE PRECISION,
		success_probability DOUBLE PRECISION,
		computed_at TIMESTAMPTZ DEFAULT NOW(),
		UNIQUE(start_slot, end_slot, top_k_builders)
	);
	
	CREATE INDEX IF NOT EXISTS idx_censorship_analysis_slots ON censorship_analysis (start_slot, end_slot);
	`

	_, err := s.db.ExecContext(ctx, schema)
	return err
}

// BatchInsertBribes inserts multiple slot bribes efficiently using COPY.
func (s *PostgresStore) BatchInsertBribes(ctx context.Context, bribes []model.SlotBribe, relayURL string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO slot_bribes (slot_number, slot_time, value_wei, value_eth, builder_pubkey, block_hash, relay_url)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (slot_time, slot_number) DO NOTHING
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Close()

	for _, bribe := range bribes {
		if bribe.ValueWei == nil {
			continue
		}

		// Convert slot to approximate timestamp (12s per slot)
		slotTime := time.Unix(1606824023, 0).Add(time.Duration(bribe.Slot*12) * time.Second)

		// Convert wei to ETH
		weiPerEth := new(big.Float).SetInt(big.NewInt(1e18))
		valueEth, _ := new(big.Float).Quo(new(big.Float).SetInt(bribe.ValueWei), weiPerEth).Float64()

		_, err := stmt.ExecContext(ctx, bribe.Slot, slotTime, bribe.ValueWei.String(), valueEth,
			bribe.BuilderPubkey, "" /* block hash */, relayURL)
		if err != nil {
			return fmt.Errorf("failed to insert bribe: %w", err)
		}
	}

	return tx.Commit()
}

// GetSlotRange retrieves bribes for a specific slot range.
func (s *PostgresStore) GetSlotRange(ctx context.Context, startSlot, endSlot uint64) ([]model.SlotBribe, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT slot_number, value_wei, builder_pubkey
		FROM slot_bribes
		WHERE slot_number BETWEEN $1 AND $2
		ORDER BY slot_number ASC
	`, startSlot, endSlot)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var bribes []model.SlotBribe
	for rows.Next() {
		var slot uint64
		var valueWeiStr string
		var builderPubkey string

		if err := rows.Scan(&slot, &valueWeiStr, &builderPubkey); err != nil {
			return nil, err
		}

		valueWei := new(big.Int)
		valueWei.SetString(valueWeiStr, 10)

		bribes = append(bribes, model.SlotBribe{
			Slot:          slot,
			ValueWei:      valueWei,
			BuilderPubkey: builderPubkey,
		})
	}

	return bribes, rows.Err()
}

// GetBuilderStats returns aggregated statistics for all builders.
func (s *PostgresStore) GetBuilderStats(ctx context.Context) ([]model.BuilderStats, error) {
	// Refresh materialized view
	if _, err := s.db.ExecContext(ctx, "REFRESH MATERIALIZED VIEW builder_stats"); err != nil {
		return nil, err
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT builder_pubkey, block_count
		FROM builder_stats
		ORDER BY block_count DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var stats []model.BuilderStats
	for rows.Next() {
		var pubkey string
		var count uint64

		if err := rows.Scan(&pubkey, &count); err != nil {
			return nil, err
		}

		stats = append(stats, model.BuilderStats{
			BuilderPubkey: pubkey,
			BlockCount:    count,
		})
	}

	return stats, rows.Err()
}

// Close closes the database connection.
func (s *PostgresStore) Close() error {
	return s.db.Close()
}
