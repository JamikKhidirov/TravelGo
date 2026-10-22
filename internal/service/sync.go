package service

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"go-travel/internal/model"
	"go-travel/internal/repository"
)

type SyncService struct {
	repo   *repository.TravelRepository
	sources []Source
}

type Source interface {
	Name() string
	FetchDestinations(context.Context) ([]model.Destination, error)
	FetchTours(context.Context) ([]model.Tour, error)
}

func NewSyncService(repo *repository.TravelRepository, sources ...Source) *SyncService {
	return &SyncService{repo: repo, sources: sources}
}

func (s *SyncService) SyncAll(ctx context.Context) error {
	for _, src := range s.sources {
		if err := s.syncSource(ctx, src); err != nil {
			return err
		}
	}
	return nil
}

func (s *SyncService) syncSource(ctx context.Context, src Source) error {
	job := &model.SyncJob{
		ID:      uuid.New(),
		Source:  src.Name(),
		Status:  "running",
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
	for _, d := range dests {
		if err := s.repo.CreateDestination(ctx, &d); err != nil {
			continue
		}
		total++
	}

	tours, err := src.FetchTours(ctx)
	if err != nil {
		s.repo.UpdateSyncJob(ctx, job.ID, "failed", total, err.Error())
		return fmt.Errorf("fetch tours from %s: %w", src.Name(), err)
	}
	for _, t := range tours {
		if err := s.repo.CreateTour(ctx, &t); err != nil {
			continue
		}
		total++
	}

	if err := s.repo.UpdateSyncJob(ctx, job.ID, "completed", total, ""); err != nil {
		return err
	}
	return nil
}

func (s *SyncService) GetJobs(ctx context.Context) ([]model.SyncJob, error) {
	// placeholder: could query repo; returning empty for now
	return []model.SyncJob{}, nil
}

var _ interface {
	SyncAll(context.Context) error
} = (*SyncService)(nil)
