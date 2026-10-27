package model

import (
	"time"

	"github.com/google/uuid"
)

type Destination struct {
	ID          uuid.UUID `json:"id" db:"id"`
	Name        string    `json:"name" db:"name" validate:"required"`
	Country     string    `json:"country" db:"country" validate:"required"`
	City        string    `json:"city" db:"city" validate:"required"`
	Description string    `json:"description" db:"description"`
	ImageURL    string    `json:"image_url" db:"image_url"`
	Rating      float64   `json:"rating" db:"rating"`
	PriceFrom   float64   `json:"price_from" db:"price_from"`
	Currency    string    `json:"currency" db:"currency"`
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time `json:"updated_at" db:"updated_at"`
	Source      string    `json:"source" db:"source"`
	ExternalID  string    `json:"external_id" db:"external_id"`
}

type Tour struct {
	ID            uuid.UUID `json:"id" db:"id"`
	DestinationID uuid.UUID `json:"destination_id" db:"destination_id" validate:"required"`
	Title         string    `json:"title" db:"title" validate:"required"`
	Description   string    `json:"description" db:"description"`
	Price         float64   `json:"price" db:"price" validate:"required,gt=0"`
	Currency      string    `json:"currency" db:"currency"`
	DurationDays  int       `json:"duration_days" db:"duration_days" validate:"required,gt=0"`
	StartDate     time.Time `json:"start_date" db:"start_date"`
	EndDate       time.Time `json:"end_date" db:"end_date"`
	Available     bool      `json:"available" db:"available"`
	ImageURL      string    `json:"image_url" db:"image_url"`
	Source        string    `json:"source" db:"source"`
	ExternalID    string    `json:"external_id" db:"external_id"`
	OwnerID       uuid.UUID `json:"owner_id" db:"owner_id"`
	CreatedAt     time.Time `json:"created_at" db:"created_at"`
	UpdatedAt     time.Time `json:"updated_at" db:"updated_at"`
}

type TravelEvent struct {
	ID          uuid.UUID `json:"id" db:"id"`
	Type        string    `json:"type" db:"type" validate:"required"` // destination_created, tour_updated, price_changed, etc.
	Aggregate   string    `json:"aggregate" db:"aggregate"`
	AggregateID uuid.UUID `json:"aggregate_id" db:"aggregate_id"`
	Payload     string    `json:"payload" db:"payload"`
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
}

type SyncJob struct {
	ID        uuid.UUID `json:"id" db:"id"`
	Source    string    `json:"source" db:"source" validate:"required"`
	Status    string    `json:"status" db:"status"` // pending, running, completed, failed
	Records   int       `json:"records" db:"records"`
	Error     string    `json:"error" db:"error"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

type Pagination struct {
	Page     int `json:"page" form:"page" validate:"min=1"`
	PageSize int `json:"page_size" form:"page_size" validate:"min=1,max=100"`
}

type Response struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
	Meta    interface{} `json:"meta,omitempty"`
}

func (p *Pagination) Offset() int {
	if p.Page < 1 {
		p.Page = 1
	}
	if p.PageSize < 1 {
		p.PageSize = 20
	}
	if p.PageSize > 100 {
		p.PageSize = 100
	}
	return (p.Page - 1) * p.PageSize
}

func (p *Pagination) Limit() int {
	if p.PageSize < 1 {
		p.PageSize = 20
	}
	if p.PageSize > 100 {
		p.PageSize = 100
	}
	return p.PageSize
}
