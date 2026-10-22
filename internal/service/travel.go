package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"go-travel/internal/model"
	"go-travel/internal/repository"
)

type TravelService struct {
	repo *repository.TravelRepository
}

func NewTravelService(repo *repository.TravelRepository) *TravelService {
	return &TravelService{repo: repo}
}

func (s *TravelService) CreateDestination(ctx context.Context, d *model.Destination) error {
	if err := validateDestination(d); err != nil {
		return err
	}
	d.ID = uuid.New()
	now := time.Now()
	d.CreatedAt = now
	d.UpdatedAt = now
	if err := s.repo.CreateDestination(ctx, d); err != nil {
		return err
	}
	return s.repo.CreateEvent(ctx, &model.TravelEvent{
		ID:          uuid.New(),
		Type:        "destination.created",
		Aggregate:   "destination",
		AggregateID: d.ID,
		Payload:     mustJSON(d),
		CreatedAt:   now,
	})
}

func (s *TravelService) ListDestinations(ctx context.Context, country, city string, p *model.Pagination) ([]model.Destination, int, error) {
	return s.repo.GetDestinations(ctx, country, city, p)
}

func (s *TravelService) GetDestination(ctx context.Context, id uuid.UUID) (*model.Destination, error) {
	return s.repo.GetDestinationByID(ctx, id)
}

func (s *TravelService) CreateTour(ctx context.Context, t *model.Tour) error {
	if err := validateTour(t); err != nil {
		return err
	}
	t.ID = uuid.New()
	now := time.Now()
	t.CreatedAt = now
	t.UpdatedAt = now
	if err := s.repo.CreateTour(ctx, t); err != nil {
		return err
	}
	return s.repo.CreateEvent(ctx, &model.TravelEvent{
		ID:          uuid.New(),
		Type:        "tour.created",
		Aggregate:   "tour",
		AggregateID: t.ID,
		Payload:     mustJSON(t),
		CreatedAt:   now,
	})
}

func (s *TravelService) ListTours(ctx context.Context, destID *uuid.UUID, available *bool, p *model.Pagination) ([]model.Tour, int, error) {
	return s.repo.GetTours(ctx, destID, available, p)
}

func (s *TravelService) GetTour(ctx context.Context, id uuid.UUID) (*model.Tour, error) {
	return s.repo.GetTourByID(ctx, id)
}

func (s *TravelService) GetEvents(ctx context.Context, aggregate string, aggregateID *uuid.UUID, limit int) ([]model.TravelEvent, error) {
	return s.repo.GetEvents(ctx, aggregate, aggregateID, limit)
}

func validateDestination(d *model.Destination) error {
	if d.Name == "" || d.Country == "" || d.City == "" {
		return fmt.Errorf("name, country and city are required")
	}
	return nil
}

func validateTour(t *model.Tour) error {
	if t.DestinationID == uuid.Nil {
		return fmt.Errorf("destination_id is required")
	}
	if t.Title == "" {
		return fmt.Errorf("title is required")
	}
	if t.Price <= 0 {
		return fmt.Errorf("price must be greater than zero")
	}
	if t.DurationDays <= 0 {
		return fmt.Errorf("duration_days must be greater than zero")
	}
	return nil
}

func mustJSON(v interface{}) string {
	b, _ := json.Marshal(v)
	return string(b)
}
