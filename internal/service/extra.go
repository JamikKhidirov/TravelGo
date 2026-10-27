package service

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"go-travel/internal/model"
	"go-travel/internal/repository"
	"golang.org/x/crypto/bcrypt"
)

type CartService struct {
	repo *repository.TravelRepository
}

func NewCartService(repo *repository.TravelRepository) *CartService {
	return &CartService{repo: repo}
}

func (s *CartService) GetOrCreateCart(ctx context.Context, userID uuid.UUID) (*model.Cart, error) {
	c, err := s.repo.GetCartByUser(ctx, userID)
	if err != nil {
		c = &model.Cart{ID: uuid.New(), UserID: userID}
		if err := s.repo.CreateCart(ctx, c); err != nil {
			return nil, err
		}
	}
	return c, nil
}

func (s *CartService) AddToCart(ctx context.Context, userID, tourID uuid.UUID, quantity int, price float64) (*model.CartItem, error) {
	cart, err := s.GetOrCreateCart(ctx, userID)
	if err != nil {
		return nil, err
	}
	item := &model.CartItem{
		ID:         uuid.New(),
		CartID:     cart.ID,
		TourID:     tourID,
		Quantity:   quantity,
		PriceAtAdd: price,
	}
	if err := s.repo.AddCartItem(ctx, item); err != nil {
		return nil, err
	}
	return item, nil
}

func (s *CartService) ListCart(ctx context.Context, userID uuid.UUID) (*model.Cart, []model.CartItem, error) {
	cart, err := s.GetOrCreateCart(ctx, userID)
	if err != nil {
		return nil, nil, err
	}
	items, err := s.repo.GetCartItems(ctx, cart.ID)
	if err != nil {
		return nil, nil, err
	}
	return cart, items, nil
}

func (s *CartService) RemoveFromCart(ctx context.Context, userID, itemID uuid.UUID) error {
	_, err := s.GetOrCreateCart(ctx, userID)
	if err != nil {
		return err
	}
	return s.repo.RemoveCartItem(ctx, itemID)
}

func (s *CartService) Checkout(ctx context.Context, userID uuid.UUID, req *model.Order) (*model.Order, error) {
	cart, items, err := s.ListCart(ctx, userID)
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return nil, errors.New("cart is empty")
	}

	total := 0.0
	for _, it := range items {
		total += it.PriceAtAdd * float64(it.Quantity)
	}

	req.ID = uuid.New()
	req.UserID = userID
	req.Status = "pending"
	req.TotalPrice = total
	req.Currency = "USD"
	req.CreatedAt = time.Now()
	req.UpdatedAt = time.Now()

	if err := s.repo.CreateOrder(ctx, req); err != nil {
		return nil, err
	}

	if err := s.repo.ClearCart(ctx, cart.ID); err != nil {
		return nil, err
	}
	return req, nil
}

type OrderService struct {
	repo *repository.TravelRepository
}

func NewOrderService(repo *repository.TravelRepository) *OrderService {
	return &OrderService{repo: repo}
}

func (s *OrderService) GetOrder(ctx context.Context, id uuid.UUID) (*model.Order, error) {
	return s.repo.GetOrderByID(ctx, id)
}

func (s *OrderService) ListOrders(ctx context.Context, userID uuid.UUID, p *model.Pagination) ([]model.Order, int, error) {
	return s.repo.GetOrdersByUser(ctx, userID, p)
}

func (s *OrderService) ListPartnerOrders(ctx context.Context, partnerID uuid.UUID, p *model.Pagination) ([]model.Order, int, error) {
	return s.repo.GetOrdersByPartner(ctx, partnerID, p)
}

func (s *OrderService) CancelOrder(ctx context.Context, id uuid.UUID) error {
	return s.repo.UpdateOrderStatus(ctx, id, "cancelled")
}

type ReviewService struct {
	repo *repository.TravelRepository
}

func NewReviewService(repo *repository.TravelRepository) *ReviewService {
	return &ReviewService{repo: repo}
}

func (s *ReviewService) AddReview(ctx context.Context, rv *model.Review) error {
	if rv.ID == uuid.Nil {
		rv.ID = uuid.New()
	}
	return s.repo.CreateReview(ctx, rv)
}

type VoucherService struct {
	repo *repository.TravelRepository
}

func NewVoucherService(repo *repository.TravelRepository) *VoucherService {
	return &VoucherService{repo: repo}
}

func (s *VoucherService) CreateVoucher(ctx context.Context, v *model.Voucher) error {
	if v.ID == uuid.Nil {
		v.ID = uuid.New()
	}
	return s.repo.CreateVoucher(ctx, v)
}
func (s *VoucherService) GetVoucher(ctx context.Context, id uuid.UUID) (*model.Voucher, error) {
	return s.repo.GetVoucherByID(ctx, id)
}

func (s *VoucherService) UpdateVoucher(ctx context.Context, v *model.Voucher) error {
	return s.repo.UpdateVoucher(ctx, v)
}

func (s *VoucherService) DeleteVoucher(ctx context.Context, id uuid.UUID) error {
	return s.repo.DeleteVoucher(ctx, id)
}

func (s *AuthService) GetUser(ctx context.Context, id uuid.UUID) (*model.User, error) {
	return s.repo.GetUserByID(ctx, id)
}

func (s *CartService) ClearCart(ctx context.Context, userID uuid.UUID) error {
	cart, err := s.GetOrCreateCart(ctx, userID)
	if err != nil {
		return err
	}
	return s.repo.ClearCart(ctx, cart.ID)
}

func (s *CartService) UpdateCartItem(ctx context.Context, userID, itemID uuid.UUID, quantity int) error {
	cart, err := s.GetOrCreateCart(ctx, userID)
	if err != nil {
		return err
	}
	return s.repo.UpdateCartItemQuantity(ctx, cart.ID, itemID, quantity)
}

func (s *OrderService) UpdateStatus(ctx context.Context, id uuid.UUID, status string) error {
	return s.repo.UpdateOrderStatus(ctx, id, status)
}

func (s *ReviewService) DeleteReview(ctx context.Context, id uuid.UUID) error {
	return s.repo.DeleteReview(ctx, id)
}

func (s *ReviewService) ListReviews(ctx context.Context, tourID uuid.UUID) ([]model.Review, error) {
	return s.repo.GetReviewsByTour(ctx, tourID)
}

func (s *VoucherService) ListVouchers(ctx context.Context, destID *uuid.UUID, p *model.Pagination) ([]model.Voucher, int, error) {
	return s.repo.GetVouchers(ctx, destID, p)
}

func (s *AuthService) ChangePassword(ctx context.Context, userID uuid.UUID, oldPassword, newPassword string) error {
	u, err := s.repo.GetUserByID(ctx, userID)
	if err != nil {
		return err
	}
	if err := bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(oldPassword)); err != nil {
		return errors.New("invalid old password")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	return s.repo.UpdateUserPassword(ctx, userID, string(hash))
}

func (s *AuthService) UpdateProfile(ctx context.Context, userID uuid.UUID, firstName, lastName, phone string) (*model.User, error) {
	u, err := s.repo.GetUserByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if firstName != "" {
		u.FirstName = firstName
	}
	if lastName != "" {
		u.LastName = lastName
	}
	if phone != "" {
		u.Phone = phone
	}
	if err := s.repo.UpdateUserProfile(ctx, userID, u.FirstName, u.LastName, u.Phone); err != nil {
		return nil, err
	}
	return s.repo.GetUserByID(ctx, userID)
}
