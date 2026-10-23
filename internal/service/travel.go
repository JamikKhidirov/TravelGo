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

func (s *TravelService) UpdateDestination(ctx context.Context, d *model.Destination) error {
	if err := validateDestination(d); err != nil {
		return err
	}
	d.UpdatedAt = time.Now()
	if err := s.repo.UpdateDestination(ctx, d); err != nil {
		return err
	}
	return s.repo.CreateEvent(ctx, &model.TravelEvent{
		ID:          uuid.New(),
		Type:        "destination.updated",
		Aggregate:   "destination",
		AggregateID: d.ID,
		Payload:     mustJSON(d),
		CreatedAt:   time.Now(),
	})
}

func (s *TravelService) DeleteDestination(ctx context.Context, id uuid.UUID) error {
	if err := s.repo.DeleteDestination(ctx, id); err != nil {
		return err
	}
	return s.repo.CreateEvent(ctx, &model.TravelEvent{
		ID:          uuid.New(),
		Type:        "destination.deleted",
		Aggregate:   "destination",
		AggregateID: id,
		Payload:     "{}",
		CreatedAt:   time.Now(),
	})
}

func (s *TravelService) SearchDestinations(ctx context.Context, q, country, city string, minRating, maxPrice float64, p *model.Pagination) ([]model.Destination, int, error) {
	return s.repo.SearchDestinations(ctx, q, country, city, minRating, maxPrice, p)
}

func (s *TravelService) UpdateTour(ctx context.Context, t *model.Tour) error {
	if err := validateTour(t); err != nil {
		return err
	}
	t.UpdatedAt = time.Now()
	if err := s.repo.UpdateTour(ctx, t); err != nil {
		return err
	}
	return s.repo.CreateEvent(ctx, &model.TravelEvent{
		ID:          uuid.New(),
		Type:        "tour.updated",
		Aggregate:   "tour",
		AggregateID: t.ID,
		Payload:     mustJSON(t),
		CreatedAt:   time.Now(),
	})
}

func (s *TravelService) DeleteTour(ctx context.Context, id uuid.UUID) error {
	if err := s.repo.DeleteTour(ctx, id); err != nil {
		return err
	}
	return s.repo.CreateEvent(ctx, &model.TravelEvent{
		ID:          uuid.New(),
		Type:        "tour.deleted",
		Aggregate:   "tour",
		AggregateID: id,
		Payload:     "{}",
		CreatedAt:   time.Now(),
	})
}

func (s *TravelService) SearchTours(ctx context.Context, q string, destID *uuid.UUID, available *bool, minPrice, maxPrice float64, minDuration, maxDuration int, p *model.Pagination) ([]model.Tour, int, error) {
	return s.repo.SearchTours(ctx, q, destID, available, minPrice, maxPrice, minDuration, maxDuration, p)
}

func (s *TravelService) TopDestinations(ctx context.Context, limit int) ([]model.Destination, error) {
	dests, _, err := s.repo.SearchDestinations(ctx, "", "", "", 0, 0, &model.Pagination{Page: 1, PageSize: limit})
	if err != nil {
		return nil, err
	}
	return dests, nil
}

func (s *TravelService) ListCountries(ctx context.Context) ([]string, error) {
	return s.repo.DistinctCountries(ctx)
}
