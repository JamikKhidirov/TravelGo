package service

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	"go-travel/internal/model"
	"go-travel/internal/repository"
)

type SyncService struct {
	repo    *repository.TravelRepository
	sources []Source
}

type Source interface {
	Name() string
	FetchDestinations(context.Context) ([]model.Destination, error)
	FetchTours(context.Context) ([]model.Tour, error)
}

type TicketSource interface {
	FetchTickets(context.Context) ([]model.Ticket, error)
}

func NewSyncService(repo *repository.TravelRepository, sources ...Source) *SyncService {
	return &SyncService{repo: repo, sources: sources}
}

func (s *SyncService) SyncAll(ctx context.Context) error {
	for _, src := range s.sources {
		if err := s.syncSource(ctx, src); err != nil {
			log.Printf("sync source %s failed: %v", src.Name(), err)
		}
	}
	return nil
}

func (s *SyncService) syncSource(ctx context.Context, src Source) error {
	job := &model.SyncJob{
		ID:        uuid.New(),
		Source:    src.Name(),
		Status:    "running",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := s.repo.CreateSyncJob(ctx, job); err != nil {
		return err
	}

	total := 0

	dests, err := src.FetchDestinations(ctx)
	if err != nil {
		s.repo.UpdateSyncJob(ctx, job.ID, "failed", total, err.Error())
		return fmt.Errorf("fetch destinations from %s: %w", src.Name(), err)
	}
	var destIDs []uuid.UUID
	for _, d := range dests {
		if err := s.repo.CreateDestination(ctx, &d); err != nil {
			continue
		}
		total++
		destIDs = append(destIDs, d.ID)
	}

	tours, err := src.FetchTours(ctx)
	if err != nil {
		s.repo.UpdateSyncJob(ctx, job.ID, "failed", total, err.Error())
		return fmt.Errorf("fetch tours from %s: %w", src.Name(), err)
	}
	for i, t := range tours {
		if len(destIDs) > 0 {
			t.DestinationID = destIDs[i%len(destIDs)]
		}
		if err := s.repo.CreateTour(ctx, &t); err != nil {
			continue
		}
		total++
	}

	if ts, ok := src.(TicketSource); ok {
		tickets, err := ts.FetchTickets(ctx)
		if err != nil {
			s.repo.UpdateSyncJob(ctx, job.ID, "failed", total, err.Error())
			return fmt.Errorf("fetch tickets from %s: %w", src.Name(), err)
		}
		for _, ticket := range tickets {
			if err := s.repo.CreateTicket(ctx, &ticket); err != nil {
				continue
			}
			total++
		}
	}

	if err := s.repo.UpdateSyncJob(ctx, job.ID, "completed", total, ""); err != nil {
		return err
	}
	return nil
}

func (s *SyncService) GetJobs(ctx context.Context) ([]model.SyncJob, error) {
	return s.repo.GetSyncJobs(ctx, 100)
}

func (s *SyncService) GetJobsBySource(ctx context.Context, source string) ([]model.SyncJob, error) {
	jobs, err := s.repo.GetSyncJobs(ctx, 100)
	if err != nil {
		return nil, err
	}
	var filtered []model.SyncJob
	for _, j := range jobs {
		if source == "" || j.Source == source {
			filtered = append(filtered, j)
		}
	}
	return filtered, nil
}

var _ interface {
	SyncAll(context.Context) error
} = (*SyncService)(nil)
