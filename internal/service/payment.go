package service

import (
	"context"
	"time"

	"github.com/google/uuid"
	"go-travel/internal/model"
	"go-travel/internal/repository"
)

type PaymentService struct {
	repo *repository.TravelRepository
}

func NewPaymentService(repo *repository.TravelRepository) *PaymentService {
	return &PaymentService{repo: repo}
}

func (s *PaymentService) CreatePayment(ctx context.Context, p *model.Payment) error {
	p.ID = uuid.New()
	p.CreatedAt = time.Now()
	return s.repo.CreatePayment(ctx, p)
}

func (s *PaymentService) GetPayment(ctx context.Context, id uuid.UUID) (*model.Payment, error) {
	return s.repo.GetPaymentByID(ctx, id)
}

func (s *PaymentService) ListPaymentsByOrder(ctx context.Context, orderID uuid.UUID) ([]model.Payment, error) {
	return s.repo.GetPaymentsByOrder(ctx, orderID)
}

func (s *PaymentService) ConfirmPayment(ctx context.Context, id uuid.UUID) error {
	return s.repo.UpdatePaymentStatus(ctx, id, "success", "txn-"+id.String())
}

func (s *PaymentService) RefundPayment(ctx context.Context, id uuid.UUID) error {
	return s.repo.UpdatePaymentStatus(ctx, id, "refunded", "")
}

func (s *PaymentService) FailPayment(ctx context.Context, id uuid.UUID) error {
	return s.repo.UpdatePaymentStatus(ctx, id, "failed", "")
}
