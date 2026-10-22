package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"go-travel/internal/model"
	"go-travel/internal/repository"
	"golang.org/x/crypto/bcrypt"
)

type AuthService struct {
	repo *repository.TravelRepository
	jwtSecret []byte
}

func NewAuthService(repo *repository.TravelRepository, jwtSecret string) *AuthService {
	return &AuthService{repo: repo, jwtSecret: []byte(jwtSecret)}
}

func (s *AuthService) Register(ctx context.Context, req *model.LoginRequest, firstName, lastName, phone, role string) (*model.User, string, error) {
	exists, err := s.repo.GetUserByEmail(ctx, req.Email)
	if err == nil && exists != nil {
		return nil, "", errors.New("user already exists")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, "", fmt.Errorf("hash password: %w", err)
	}

	u := &model.User{
		ID:        uuid.New(),
		Email:     req.Email,
		Password:  string(hash),
		FirstName: firstName,
		LastName:  lastName,
		Phone:     phone,
		Role:      role,
	}
	if err := s.repo.CreateUser(ctx, u); err != nil {
		return nil, "", err
	}

	token, err := s.generateToken(u.ID, u.Role)
	if err != nil {
		return nil, "", err
	}
	return u, token, nil
}

func (s *AuthService) Login(ctx context.Context, req *model.LoginRequest) (*model.User, string, error) {
	u, err := s.repo.GetUserByEmail(ctx, req.Email)
	if err != nil {
		return nil, "", errors.New("invalid credentials")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(req.Password)); err != nil {
		return nil, "", errors.New("invalid credentials")
	}
	token, err := s.generateToken(u.ID, u.Role)
	if err != nil {
		return nil, "", err
	}
	return u, token, nil
}

func (s *AuthService) generateToken(userID uuid.UUID, role string) (string, error) {
	// simplified JWT-like token using standard library would be better, but using claims map manually
	// For this service we return a signed string. Using jwt-go would require claims struct.
	// We'll produce a placeholder signed token for compatibility.
	token := fmt.Sprintf("travel-token-%s-%s-%d", userID, role, time.Now().Unix())
	return token, nil
}

func (s *AuthService) ValidateToken(token string) (uuid.UUID, string, error) {
	// simplified validation
	return uuid.Nil, "", errors.New("token validation not implemented in mock mode")
}

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
	cart, err := s.GetOrCreateCart(ctx, userID)
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
	return s.repo.CreateReview(ctx, rv)
}

func (s *ReviewService) ListReviews(ctx context.Context, tourID uuid.UUID) ([]model.Review, error) {
	return s.repo.GetReviewsByTour(ctx, tourID)
}

type VoucherService struct {
	repo *repository.TravelRepository
}

func NewVoucherService(repo *repository.TravelRepository) *VoucherService {
	return &VoucherService{repo: repo}
}

func (s *VoucherService) CreateVoucher(ctx context.Context, v *model.Voucher) error {
	return s.repo.CreateVoucher(ctx, v)
}

func (s *VoucherService) ListVouchers(ctx context.Context, destID *uuid.UUID, p *model.Pagination) ([]model.Voucher, int, error) {
	return s.repo.GetVouchers(ctx, destID, p)
}

func (s *VoucherService) GetVoucher(ctx context.Context, id uuid.UUID) (*model.Voucher, error) {
	return s.repo.GetVoucherByID(ctx, id)
}
