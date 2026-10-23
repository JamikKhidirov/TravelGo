package model

import (
	"time"

	"github.com/google/uuid"
)

type Ticket struct {
	ID                 uuid.UUID `json:"id" db:"id"`
	Type               string    `json:"type" db:"type" binding:"required,oneof=flight train bus"`
	Provider           string    `json:"provider" db:"provider"`
	OriginCity         string    `json:"origin_city" db:"origin_city" binding:"required"`
	OriginCountry      string    `json:"origin_country" db:"origin_country"`
	DestinationCity    string    `json:"destination_city" db:"destination_city" binding:"required"`
	DestinationCountry string    `json:"destination_country" db:"destination_country"`
	DepartureAt        time.Time `json:"departure_at" db:"departure_at" binding:"required"`
	ArrivalAt          time.Time `json:"arrival_at" db:"arrival_at" binding:"required"`
	Price              float64   `json:"price" db:"price" binding:"required,gt=0"`
	Currency           string    `json:"currency" db:"currency"`
	Carrier            string    `json:"carrier" db:"carrier"`
	Number             string    `json:"number" db:"number"`
	Class              string    `json:"class" db:"class"`
	AvailableSeats     int       `json:"available_seats" db:"available_seats"`
	Source             string    `json:"source" db:"source"`
	ExternalID         string    `json:"external_id" db:"external_id"`
	CreatedAt          time.Time `json:"created_at" db:"created_at"`
	UpdatedAt          time.Time `json:"updated_at" db:"updated_at"`
}

type DeviceToken struct {
	ID         uuid.UUID `json:"id" db:"id"`
	UserID     uuid.UUID `json:"user_id" db:"user_id"`
	Platform   string    `json:"platform" db:"platform" validate:"required,oneof=ios android web"`
	Token      string    `json:"token" db:"token" binding:"required"`
	AppVersion string    `json:"app_version" db:"app_version"`
	IsActive   bool      `json:"is_active" db:"is_active"`
	CreatedAt  time.Time `json:"created_at" db:"created_at"`
	UpdatedAt  time.Time `json:"updated_at" db:"updated_at"`
}

type PushNotification struct {
	ID          uuid.UUID `json:"id" db:"id"`
	UserID      uuid.UUID `json:"user_id" db:"user_id"`
	Title       string    `json:"title" db:"title"`
	Body        string    `json:"body" db:"body"`
	Data        string    `json:"data" db:"data"`
	Platform    string    `json:"platform" db:"platform"`
	Status      string    `json:"status" db:"status"`
	SentCount   int       `json:"sent_count" db:"sent_count"`
	FailedCount int       `json:"failed_count" db:"failed_count"`
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
}
