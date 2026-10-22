package adapter

import (
	"database/sql"

	_ "github.com/lib/pq"
)

type PostgresAdapter struct {
	db *sql.DB
}

func NewPostgresAdapter(dsn string) (*PostgresAdapter, error) {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, err
	}
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, err
	}
	return &PostgresAdapter{db: db}, nil
}

func (p *PostgresAdapter) DB() *sql.DB {
	return p.db
}

func (p *PostgresAdapter) Close() error {
	return p.db.Close()
}
