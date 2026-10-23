package model

import (
	"time"

	"github.com/google/uuid"
)

type User struct {
	ID        uuid.UUID `json:"id" db:"id"`
	Email     string    `json:"email" db:"email" validate:"required,email"`
	Password  string    `json:"-" db:"password_hash" validate:"required,min=6"`
	FirstName string    `json:"first_name" db:"first_name" validate:"required"`
	LastName  string    `json:"last_name" db:"last_name" validate:"required"`
	Phone     string    `json:"phone" db:"phone"`
	Role      string    `json:"role" db:"role" validate:"required,oneof=user admin"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

type LoginRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required"`
}

type Cart struct {
	ID        uuid.UUID `json:"id" db:"id"`
	UserID    uuid.UUID `json:"user_id" db:"user_id" validate:"required"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

type CartItem struct {
	ID         uuid.UUID `json:"id" db:"id"`
	CartID     uuid.UUID `json:"cart_id" db:"cart_id"`
	TourID     uuid.UUID `json:"tour_id" db:"tour_id" validate:"required"`
	Quantity   int       `json:"quantity" db:"quantity" validate:"min=1"`
	PriceAtAdd float64   `json:"price_at_add" db:"price_at_add"`
	CreatedAt  time.Time `json:"created_at" db:"created_at"`
}

type Order struct {
	ID             uuid.UUID `json:"id" db:"id"`
	UserID         uuid.UUID `json:"user_id" db:"user_id" validate:"required"`
	TourID         uuid.UUID `json:"tour_id" db:"tour_id" validate:"required"`
	DestinationID  uuid.UUID `json:"destination_id" db:"destination_id" validate:"required"`
	Status         string    `json:"status" db:"status" validate:"required,oneof=pending paid cancelled completed"`
	TotalPrice     float64   `json:"total_price" db:"total_price" validate:"gt=0"`
	Currency       string    `json:"currency" db:"currency"`
	PassengerName  string    `json:"passenger_name" db:"passenger_name" validate:"required"`
	PassengerEmail string    `json:"passenger_email" db:"passenger_email" validate:"required,email"`
	PassengerPhone string    `json:"passenger_phone" db:"passenger_phone"`
	TravelDate     time.Time `json:"travel_date" db:"travel_date" validate:"required"`
	Nights         int       `json:"nights" db:"nights" validate:"min=1"`
	Adults         int       `json:"adults" db:"adults" validate:"min=1"`
	Children       int       `json:"children" db:"children"`
	CreatedAt      time.Time `json:"created_at" db:"created_at"`
	UpdatedAt      time.Time `json:"updated_at" db:"updated_at"`
}

type Review struct {
	ID        uuid.UUID `json:"id" db:"id"`
	TourID    uuid.UUID `json:"tour_id" db:"tour_id" validate:"required"`
	UserID    uuid.UUID `json:"user_id" db:"user_id" validate:"required"`
	Rating    int       `json:"rating" db:"rating" validate:"min=1,max=5"`
	Comment   string    `json:"comment" db:"comment"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}

type Payment struct {
	ID            uuid.UUID `json:"id" db:"id"`
	OrderID       uuid.UUID `json:"order_id" db:"order_id" validate:"required"`
	Amount        float64   `json:"amount" db:"amount" validate:"gt=0"`
	Currency      string    `json:"currency" db:"currency"`
	Status        string    `json:"status" db:"status" validate:"required,oneof=pending success failed"`
	Provider      string    `json:"provider" db:"provider"`
	TransactionID string    `json:"transaction_id" db:"transaction_id"`
	CreatedAt     time.Time `json:"created_at" db:"created_at"`
}

type Voucher struct {
	ID             uuid.UUID `json:"id" db:"id"`
	Title          string    `json:"title" db:"title" validate:"required"`
	Description    string    `json:"description" db:"description"`
	DestinationID  uuid.UUID `json:"destination_id" db:"destination_id" validate:"required"`
	Price          float64   `json:"price" db:"price" validate:"gt=0"`
	Currency       string    `json:"currency" db:"currency"`
	DurationNights int       `json:"duration_nights" db:"duration_nights" validate:"min=1"`
	IncludesFlight bool      `json:"includes_flight" db:"includes_flight"`
	IncludesHotel  bool      `json:"includes_hotel" db:"includes_hotel"`
	IncludesMeal   bool      `json:"includes_meal" db:"includes_meal"`
	ImageURL       string    `json:"image_url" db:"image_url"`
	IsActive       bool      `json:"is_active" db:"is_active"`
	CreatedAt      time.Time `json:"created_at" db:"created_at"`
}
