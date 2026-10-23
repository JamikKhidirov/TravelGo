package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"go-travel/internal/model"
)

type TravelRepository struct {
	db *sqlx.DB
}

func NewTravelRepository(db *sqlx.DB) *TravelRepository {
	return &TravelRepository{db: db}
}

func (r *TravelRepository) Migrate(ctx context.Context) error {
	schema := `
	CREATE TABLE IF NOT EXISTS destinations (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		name VARCHAR(255) NOT NULL,
		country VARCHAR(100) NOT NULL,
		city VARCHAR(100) NOT NULL,
		description TEXT,
		image_url VARCHAR(500),
		rating NUMERIC(3,2) DEFAULT 0,
		price_from NUMERIC(10,2) DEFAULT 0,
		currency VARCHAR(10) DEFAULT 'USD',
		created_at TIMESTAMPTZ DEFAULT NOW(),
		updated_at TIMESTAMPTZ DEFAULT NOW(),
		source VARCHAR(100),
		external_id VARCHAR(255),
		UNIQUE(source, external_id)
	);

	CREATE TABLE IF NOT EXISTS tours (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		destination_id UUID REFERENCES destinations(id),
		title VARCHAR(255) NOT NULL,
		description TEXT,
		price NUMERIC(10,2) NOT NULL,
		currency VARCHAR(10) DEFAULT 'USD',
		duration_days INTEGER NOT NULL,
		start_date TIMESTAMPTZ NOT NULL,
		end_date TIMESTAMPTZ NOT NULL,
		available BOOLEAN DEFAULT TRUE,
		image_url VARCHAR(500),
		source VARCHAR(100),
		external_id VARCHAR(255),
		created_at TIMESTAMPTZ DEFAULT NOW(),
		updated_at TIMESTAMPTZ DEFAULT NOW(),
		UNIQUE(source, external_id)
	);

	CREATE TABLE IF NOT EXISTS travel_events (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		type VARCHAR(100) NOT NULL,
		aggregate VARCHAR(100) NOT NULL,
		aggregate_id UUID NOT NULL,
		payload TEXT NOT NULL,
		created_at TIMESTAMPTZ DEFAULT NOW()
	);

	CREATE TABLE IF NOT EXISTS sync_jobs (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		source VARCHAR(100) NOT NULL,
		status VARCHAR(50) DEFAULT 'pending',
		records INTEGER DEFAULT 0,
		error TEXT,
		created_at TIMESTAMPTZ DEFAULT NOW(),
		updated_at TIMESTAMPTZ DEFAULT NOW()
	);

	CREATE INDEX IF NOT EXISTS idx_destinations_country_city ON destinations(country, city);
	CREATE INDEX IF NOT EXISTS idx_destinations_source_ext ON destinations(source, external_id);
	CREATE INDEX IF NOT EXISTS idx_tours_destination ON tours(destination_id);
	CREATE INDEX IF NOT EXISTS idx_tours_available ON tours(available);
	CREATE INDEX IF NOT EXISTS idx_tours_source_ext ON tours(source, external_id);
	CREATE INDEX IF NOT EXISTS idx_travel_events_type ON travel_events(type);
	CREATE INDEX IF NOT EXISTS idx_travel_events_created ON travel_events(created_at);
	CREATE INDEX IF NOT EXISTS idx_sync_jobs_status ON sync_jobs(status);
	`

	_, err := r.db.ExecContext(ctx, schema)
	return err
}

func (r *TravelRepository) CreateDestination(ctx context.Context, d *model.Destination) error {
	query := `
		INSERT INTO destinations (id, name, country, city, description, image_url, rating, price_from, currency, source, external_id)
		VALUES (:id, :name, :country, :city, :description, :image_url, :rating, :price_from, :currency, :source, :external_id)
		ON CONFLICT (source, external_id) DO UPDATE SET
			name = EXCLUDED.name,
			country = EXCLUDED.country,
			city = EXCLUDED.city,
			description = EXCLUDED.description,
			image_url = EXCLUDED.image_url,
			rating = EXCLUDED.rating,
			price_from = EXCLUDED.price_from,
			currency = EXCLUDED.currency,
			updated_at = NOW()
		RETURNING id, created_at, updated_at
	`
	rows, err := r.db.NamedQueryContext(ctx, query, d)
	if err != nil {
		return fmt.Errorf("create destination: %w", err)
	}
	defer rows.Close()
	if rows.Next() {
		return rows.Scan(&d.ID, &d.CreatedAt, &d.UpdatedAt)
	}
	return fmt.Errorf("no row returned")
}

func (r *TravelRepository) GetDestinations(ctx context.Context, country, city string, p *model.Pagination) ([]model.Destination, int, error) {
	var destinations []model.Destination

	where := "WHERE 1=1"
	var args []interface{}
	argIdx := 1
	if country != "" {
		where += fmt.Sprintf(" AND country = $%d", argIdx)
		args = append(args, country)
		argIdx++
	}
	if city != "" {
		where += fmt.Sprintf(" AND city = $%d", argIdx)
		args = append(args, city)
		argIdx++
	}

	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM destinations %s", where)
	var total int
	if err := r.db.GetContext(ctx, &total, r.db.Rebind(countQuery), args...); err != nil {
		return nil, 0, fmt.Errorf("count destinations: %w", err)
	}

	query := fmt.Sprintf("SELECT * FROM destinations %s ORDER BY created_at DESC LIMIT $%d OFFSET $%d", where, argIdx, argIdx+1)
	args = append(args, p.Limit(), p.Offset())
	if err := r.db.SelectContext(ctx, &destinations, r.db.Rebind(query), args...); err != nil {
		return nil, 0, fmt.Errorf("select destinations: %w", err)
	}

	return destinations, total, nil
}

func (r *TravelRepository) GetDestinationByID(ctx context.Context, id uuid.UUID) (*model.Destination, error) {
	var d model.Destination
	query := "SELECT * FROM destinations WHERE id = $1"
	if err := r.db.GetContext(ctx, &d, r.db.Rebind(query), id); err != nil {
		return nil, fmt.Errorf("get destination: %w", err)
	}
	return &d, nil
}

func (r *TravelRepository) CreateTour(ctx context.Context, t *model.Tour) error {
	query := `
		INSERT INTO tours (id, destination_id, title, description, price, currency, duration_days, start_date, end_date, available, image_url, source, external_id)
		VALUES (:id, :destination_id, :title, :description, :price, :currency, :duration_days, :start_date, :end_date, :available, :image_url, :source, :external_id)
		ON CONFLICT (source, external_id) DO UPDATE SET
			destination_id = EXCLUDED.destination_id,
			title = EXCLUDED.title,
			description = EXCLUDED.description,
			price = EXCLUDED.price,
			currency = EXCLUDED.currency,
			duration_days = EXCLUDED.duration_days,
			start_date = EXCLUDED.start_date,
			end_date = EXCLUDED.end_date,
			available = EXCLUDED.available,
			image_url = EXCLUDED.image_url,
			updated_at = NOW()
		RETURNING id, created_at, updated_at
	`
	rows, err := r.db.NamedQueryContext(ctx, query, t)
	if err != nil {
		return fmt.Errorf("create tour: %w", err)
	}
	defer rows.Close()
	if rows.Next() {
		return rows.Scan(&t.ID, &t.CreatedAt, &t.UpdatedAt)
	}
	return fmt.Errorf("no row returned")
}

func (r *TravelRepository) GetTours(ctx context.Context, destID *uuid.UUID, available *bool, p *model.Pagination) ([]model.Tour, int, error) {
	var tours []model.Tour

	where := "WHERE 1=1"
	var args []interface{}
	argIdx := 1
	if destID != nil {
		where += fmt.Sprintf(" AND destination_id = $%d", argIdx)
		args = append(args, *destID)
		argIdx++
	}
	if available != nil {
		where += fmt.Sprintf(" AND available = $%d", argIdx)
		args = append(args, *available)
		argIdx++
	}

	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM tours %s", where)
	var total int
	if err := r.db.GetContext(ctx, &total, r.db.Rebind(countQuery), args...); err != nil {
		return nil, 0, fmt.Errorf("count tours: %w", err)
	}

	query := fmt.Sprintf("SELECT * FROM tours %s ORDER BY created_at DESC LIMIT $%d OFFSET $%d", where, argIdx, argIdx+1)
	args = append(args, p.Limit(), p.Offset())
	if err := r.db.SelectContext(ctx, &tours, r.db.Rebind(query), args...); err != nil {
		return nil, 0, fmt.Errorf("select tours: %w", err)
	}

	return tours, total, nil
}

func (r *TravelRepository) GetTourByID(ctx context.Context, id uuid.UUID) (*model.Tour, error) {
	var t model.Tour
	query := "SELECT * FROM tours WHERE id = $1"
	if err := r.db.GetContext(ctx, &t, r.db.Rebind(query), id); err != nil {
		return nil, fmt.Errorf("get tour: %w", err)
	}
	return &t, nil
}

func (r *TravelRepository) CreateEvent(ctx context.Context, e *model.TravelEvent) error {
	query := `
		INSERT INTO travel_events (id, type, aggregate, aggregate_id, payload)
		VALUES (:id, :type, :aggregate, :aggregate_id, :payload)
		RETURNING id, created_at
	`
	rows, err := r.db.NamedQueryContext(ctx, query, e)
	if err != nil {
		return fmt.Errorf("create event: %w", err)
	}
	defer rows.Close()
	if rows.Next() {
		return rows.Scan(&e.ID, &e.CreatedAt)
	}
	return fmt.Errorf("no row returned")
}

func (r *TravelRepository) GetEvents(ctx context.Context, aggregate string, aggregateID *uuid.UUID, limit int) ([]model.TravelEvent, error) {
	var events []model.TravelEvent
	query := "SELECT * FROM travel_events WHERE 1=1"
	args := []interface{}{}
	argIdx := 1

	if aggregate != "" {
		query += fmt.Sprintf(" AND aggregate = $%d", argIdx)
		args = append(args, aggregate)
		argIdx++
	}
	if aggregateID != nil {
		query += fmt.Sprintf(" AND aggregate_id = $%d", argIdx)
		args = append(args, *aggregateID)
		argIdx++
	}
	query += fmt.Sprintf(" ORDER BY created_at DESC LIMIT $%d", argIdx)
	args = append(args, limit)

	if err := r.db.SelectContext(ctx, &events, r.db.Rebind(query), args...); err != nil {
		return nil, fmt.Errorf("get events: %w", err)
	}
	return events, nil
}

func (r *TravelRepository) CreateSyncJob(ctx context.Context, j *model.SyncJob) error {
	query := `
		INSERT INTO sync_jobs (id, source, status, records, error)
		VALUES (:id, :source, :status, :records, :error)
		RETURNING id, created_at, updated_at
	`
	rows, err := r.db.NamedQueryContext(ctx, query, j)
	if err != nil {
		return fmt.Errorf("create sync job: %w", err)
	}
	defer rows.Close()
	if rows.Next() {
		return rows.Scan(&j.ID, &j.CreatedAt, &j.UpdatedAt)
	}
	return fmt.Errorf("no row returned")
}

func (r *TravelRepository) UpdateSyncJob(ctx context.Context, id uuid.UUID, status string, records int, errStr string) error {
	query := `
		UPDATE sync_jobs
		SET status = $1, records = $2, error = $3, updated_at = NOW()
		WHERE id = $4
	`
	_, err := r.db.ExecContext(ctx, r.db.Rebind(query), status, records, errStr, id)
	return err
}
