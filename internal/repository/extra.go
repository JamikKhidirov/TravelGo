package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"go-travel/internal/model"
)

func (r *TravelRepository) MigrateExtra(ctx context.Context) error {
	schema := `
	CREATE TABLE IF NOT EXISTS users (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		email VARCHAR(255) UNIQUE NOT NULL,
		password_hash VARCHAR(255) NOT NULL,
		first_name VARCHAR(100) NOT NULL,
		last_name VARCHAR(100) NOT NULL,
		phone VARCHAR(50),
		role VARCHAR(20) DEFAULT 'user',
		created_at TIMESTAMPTZ DEFAULT NOW(),
		updated_at TIMESTAMPTZ DEFAULT NOW()
	);

	CREATE TABLE IF NOT EXISTS carts (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		user_id UUID UNIQUE REFERENCES users(id),
		created_at TIMESTAMPTZ DEFAULT NOW(),
		updated_at TIMESTAMPTZ DEFAULT NOW()
	);

	CREATE TABLE IF NOT EXISTS cart_items (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		cart_id UUID REFERENCES carts(id) ON DELETE CASCADE,
		tour_id UUID REFERENCES tours(id) ON DELETE CASCADE,
		quantity INTEGER DEFAULT 1,
		price_at_add NUMERIC(10,2) DEFAULT 0,
		created_at TIMESTAMPTZ DEFAULT NOW()
	);

	CREATE TABLE IF NOT EXISTS orders (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		user_id UUID REFERENCES users(id),
		tour_id UUID REFERENCES tours(id),
		destination_id UUID REFERENCES destinations(id),
		status VARCHAR(50) DEFAULT 'pending',
		total_price NUMERIC(10,2) NOT NULL,
		currency VARCHAR(10) DEFAULT 'USD',
		passenger_name VARCHAR(255) NOT NULL,
		passenger_email VARCHAR(255) NOT NULL,
		passenger_phone VARCHAR(50),
		travel_date TIMESTAMPTZ NOT NULL,
		nights INTEGER NOT NULL,
		adults INTEGER NOT NULL DEFAULT 1,
		children INTEGER DEFAULT 0,
		created_at TIMESTAMPTZ DEFAULT NOW(),
		updated_at TIMESTAMPTZ DEFAULT NOW()
	);

	CREATE TABLE IF NOT EXISTS reviews (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		tour_id UUID REFERENCES tours(id) ON DELETE CASCADE,
		user_id UUID REFERENCES users(id) ON DELETE CASCADE,
		rating INTEGER CHECK (rating >= 1 AND rating <= 5),
		comment TEXT,
		created_at TIMESTAMPTZ DEFAULT NOW()
	);

	CREATE TABLE IF NOT EXISTS payments (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		order_id UUID REFERENCES orders(id) ON DELETE CASCADE,
		amount NUMERIC(10,2) NOT NULL,
		currency VARCHAR(10) DEFAULT 'USD',
		status VARCHAR(50) DEFAULT 'pending',
		provider VARCHAR(100),
		transaction_id VARCHAR(255),
		created_at TIMESTAMPTZ DEFAULT NOW()
	);

	CREATE TABLE IF NOT EXISTS vouchers (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		title VARCHAR(255) NOT NULL,
		description TEXT,
		destination_id UUID REFERENCES destinations(id),
		price NUMERIC(10,2) NOT NULL,
		currency VARCHAR(10) DEFAULT 'USD',
		duration_nights INTEGER NOT NULL,
		includes_flight BOOLEAN DEFAULT FALSE,
		includes_hotel BOOLEAN DEFAULT FALSE,
		includes_meal BOOLEAN DEFAULT FALSE,
		image_url VARCHAR(500),
		is_active BOOLEAN DEFAULT TRUE,
		created_at TIMESTAMPTZ DEFAULT NOW()
	);

	CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);
	CREATE INDEX IF NOT EXISTS idx_carts_user ON carts(user_id);
	CREATE INDEX IF NOT EXISTS idx_cart_items_cart ON cart_items(cart_id);
	CREATE INDEX IF NOT EXISTS idx_orders_user ON orders(user_id);
	CREATE INDEX IF NOT EXISTS idx_orders_status ON orders(status);
	CREATE INDEX IF NOT EXISTS idx_reviews_tour ON reviews(tour_id);
	CREATE INDEX IF NOT EXISTS idx_payments_order ON payments(order_id);
	CREATE INDEX IF NOT EXISTS idx_vouchers_destination ON vouchers(destination_id);
	`

	_, err := r.db.ExecContext(ctx, schema)
	return err
}

func (r *TravelRepository) CreateUser(ctx context.Context, u *model.User) error {
	query := `
		INSERT INTO users (id, email, password_hash, first_name, last_name, phone, role)
		VALUES (:id, :email, :password_hash, :first_name, :last_name, :phone, :role)
		RETURNING id, created_at, updated_at
	`
	rows, err := r.db.NamedQueryContext(ctx, query, u)
	if err != nil {
		return fmt.Errorf("create user: %w", err)
	}
	defer rows.Close()
	if rows.Next() {
		return rows.Scan(&u.ID, &u.CreatedAt, &u.UpdatedAt)
	}
	return fmt.Errorf("no row returned")
}

func (r *TravelRepository) GetUserByEmail(ctx context.Context, email string) (*model.User, error) {
	var u model.User
	query := "SELECT * FROM users WHERE email = $1"
	if err := r.db.GetContext(ctx, &u, r.db.Rebind(query), email); err != nil {
		return nil, fmt.Errorf("get user: %w", err)
	}
	return &u, nil
}

func (r *TravelRepository) GetUserByID(ctx context.Context, id uuid.UUID) (*model.User, error) {
	var u model.User
	query := "SELECT * FROM users WHERE id = $1"
	if err := r.db.GetContext(ctx, &u, r.db.Rebind(query), id); err != nil {
		return nil, fmt.Errorf("get user: %w", err)
	}
	return &u, nil
}

func (r *TravelRepository) GetCartByUser(ctx context.Context, userID uuid.UUID) (*model.Cart, error) {
	var c model.Cart
	query := "SELECT * FROM carts WHERE user_id = $1"
	if err := r.db.GetContext(ctx, &c, r.db.Rebind(query), userID); err != nil {
		return nil, fmt.Errorf("get cart: %w", err)
	}
	return &c, nil
}

func (r *TravelRepository) CreateCart(ctx context.Context, c *model.Cart) error {
	query := `
		INSERT INTO carts (id, user_id)
		VALUES (:id, :user_id)
		RETURNING id, created_at, updated_at
	`
	rows, err := r.db.NamedQueryContext(ctx, query, c)
	if err != nil {
		return fmt.Errorf("create cart: %w", err)
	}
	defer rows.Close()
	if rows.Next() {
		return rows.Scan(&c.ID, &c.CreatedAt, &c.UpdatedAt)
	}
	return fmt.Errorf("no row returned")
}

func (r *TravelRepository) AddCartItem(ctx context.Context, item *model.CartItem) error {
	query := `
		INSERT INTO cart_items (id, cart_id, tour_id, quantity, price_at_add)
		VALUES (:id, :cart_id, :tour_id, :quantity, :price_at_add)
		RETURNING id, created_at
	`
	rows, err := r.db.NamedQueryContext(ctx, query, item)
	if err != nil {
		return fmt.Errorf("add cart item: %w", err)
	}
	defer rows.Close()
	if rows.Next() {
		return rows.Scan(&item.ID, &item.CreatedAt)
	}
	return fmt.Errorf("no row returned")
}

func (r *TravelRepository) GetCartItems(ctx context.Context, cartID uuid.UUID) ([]model.CartItem, error) {
	var items []model.CartItem
	query := "SELECT * FROM cart_items WHERE cart_id = $1 ORDER BY created_at DESC"
	if err := r.db.SelectContext(ctx, &items, r.db.Rebind(query), cartID); err != nil {
		return nil, fmt.Errorf("get cart items: %w", err)
	}
	return items, nil
}

func (r *TravelRepository) RemoveCartItem(ctx context.Context, itemID uuid.UUID) error {
	query := "DELETE FROM cart_items WHERE id = $1"
	_, err := r.db.ExecContext(ctx, r.db.Rebind(query), itemID)
	return err
}

func (r *TravelRepository) ClearCart(ctx context.Context, cartID uuid.UUID) error {
	query := "DELETE FROM cart_items WHERE cart_id = $1"
	_, err := r.db.ExecContext(ctx, r.db.Rebind(query), cartID)
	return err
}

func (r *TravelRepository) CreateOrder(ctx context.Context, o *model.Order) error {
	query := `
		INSERT INTO orders (id, user_id, tour_id, destination_id, status, total_price, currency, passenger_name, passenger_email, passenger_phone, travel_date, nights, adults, children)
		VALUES (:id, :user_id, :tour_id, :destination_id, :status, :total_price, :currency, :passenger_name, :passenger_email, :passenger_phone, :travel_date, :nights, :adults, :children)
		RETURNING id, created_at, updated_at
	`
	rows, err := r.db.NamedQueryContext(ctx, query, o)
	if err != nil {
		return fmt.Errorf("create order: %w", err)
	}
	defer rows.Close()
	if rows.Next() {
		return rows.Scan(&o.ID, &o.CreatedAt, &o.UpdatedAt)
	}
	return fmt.Errorf("no row returned")
}

func (r *TravelRepository) GetOrderByID(ctx context.Context, id uuid.UUID) (*model.Order, error) {
	var o model.Order
	query := "SELECT * FROM orders WHERE id = $1"
	if err := r.db.GetContext(ctx, &o, r.db.Rebind(query), id); err != nil {
		return nil, fmt.Errorf("get order: %w", err)
	}
	return &o, nil
}

func (r *TravelRepository) GetOrdersByUser(ctx context.Context, userID uuid.UUID, p *model.Pagination) ([]model.Order, int, error) {
	var orders []model.Order
	query := "SELECT * FROM orders WHERE user_id = $1 ORDER BY created_at DESC LIMIT $2 OFFSET $3"
	offset := p.Offset()
	limit := p.Limit()
	if err := r.db.SelectContext(ctx, &orders, r.db.Rebind(query), userID, limit, offset); err != nil {
		return nil, 0, fmt.Errorf("get orders: %w", err)
	}
	var total int
	countQuery := "SELECT COUNT(*) FROM orders WHERE user_id = $1"
	if err := r.db.GetContext(ctx, &total, r.db.Rebind(countQuery), userID); err != nil {
		return nil, 0, fmt.Errorf("count orders: %w", err)
	}
	return orders, total, nil
}

func (r *TravelRepository) UpdateOrderStatus(ctx context.Context, id uuid.UUID, status string) error {
	query := "UPDATE orders SET status = $1, updated_at = NOW() WHERE id = $2"
	_, err := r.db.ExecContext(ctx, r.db.Rebind(query), status, id)
	return err
}

func (r *TravelRepository) CreateReview(ctx context.Context, rv *model.Review) error {
	query := `
		INSERT INTO reviews (id, tour_id, user_id, rating, comment)
		VALUES (:id, :tour_id, :user_id, :rating, :comment)
		RETURNING id, created_at
	`
	rows, err := r.db.NamedQueryContext(ctx, query, rv)
	if err != nil {
		return fmt.Errorf("create review: %w", err)
	}
	defer rows.Close()
	if rows.Next() {
		return rows.Scan(&rv.ID, &rv.CreatedAt)
	}
	return fmt.Errorf("no row returned")
}

func (r *TravelRepository) GetReviewsByTour(ctx context.Context, tourID uuid.UUID) ([]model.Review, error) {
	var reviews []model.Review
	query := "SELECT * FROM reviews WHERE tour_id = $1 ORDER BY created_at DESC"
	if err := r.db.SelectContext(ctx, &reviews, r.db.Rebind(query), tourID); err != nil {
		return nil, fmt.Errorf("get reviews: %w", err)
	}
	return reviews, nil
}

func (r *TravelRepository) CreatePayment(ctx context.Context, p *model.Payment) error {
	query := `
		INSERT INTO payments (id, order_id, amount, currency, status, provider, transaction_id)
		VALUES (:id, :order_id, :amount, :currency, :status, :provider, :transaction_id)
		RETURNING id, created_at
	`
	rows, err := r.db.NamedQueryContext(ctx, query, p)
	if err != nil {
		return fmt.Errorf("create payment: %w", err)
	}
	defer rows.Close()
	if rows.Next() {
		return rows.Scan(&p.ID, &p.CreatedAt)
	}
	return fmt.Errorf("no row returned")
}

func (r *TravelRepository) CreateVoucher(ctx context.Context, v *model.Voucher) error {
	query := `
		INSERT INTO vouchers (id, title, description, destination_id, price, currency, duration_nights, includes_flight, includes_hotel, includes_meal, image_url, is_active)
		VALUES (:id, :title, :description, :destination_id, :price, :currency, :duration_nights, :includes_flight, :includes_hotel, :includes_meal, :image_url, :is_active)
		RETURNING id, created_at
	`
	rows, err := r.db.NamedQueryContext(ctx, query, v)
	if err != nil {
		return fmt.Errorf("create voucher: %w", err)
	}
	defer rows.Close()
	if rows.Next() {
		return rows.Scan(&v.ID, &v.CreatedAt)
	}
	return fmt.Errorf("no row returned")
}

func (r *TravelRepository) GetVouchers(ctx context.Context, destID *uuid.UUID, p *model.Pagination) ([]model.Voucher, int, error) {
	var vouchers []model.Voucher
	where := "WHERE is_active = true"
	args := []interface{}{}
	argIdx := 1
	if destID != nil {
		where += fmt.Sprintf(" AND destination_id = $%d", argIdx)
		args = append(args, *destID)
		argIdx++
	}

	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM vouchers %s", where)
	var total int
	if err := r.db.GetContext(ctx, &total, r.db.Rebind(countQuery), args...); err != nil {
		return nil, 0, fmt.Errorf("count vouchers: %w", err)
	}

	query := fmt.Sprintf("SELECT * FROM vouchers %s ORDER BY created_at DESC LIMIT $%d OFFSET $%d", where, argIdx, argIdx+1)
	args = append(args, p.Limit(), p.Offset())
	if err := r.db.SelectContext(ctx, &vouchers, r.db.Rebind(query), args...); err != nil {
		return nil, 0, fmt.Errorf("get vouchers: %w", err)
	}
	return vouchers, total, nil
}

func (r *TravelRepository) GetVoucherByID(ctx context.Context, id uuid.UUID) (*model.Voucher, error) {
	var v model.Voucher
	query := "SELECT * FROM vouchers WHERE id = $1"
	if err := r.db.GetContext(ctx, &v, r.db.Rebind(query), id); err != nil {
		return nil, fmt.Errorf("get voucher: %w", err)
	}
	return &v, nil
}

func (r *TravelRepository) GetPaymentByID(ctx context.Context, id uuid.UUID) (*model.Payment, error) {
	var p model.Payment
	query := "SELECT * FROM payments WHERE id = $1"
	if err := r.db.GetContext(ctx, &p, r.db.Rebind(query), id); err != nil {
		return nil, fmt.Errorf("get payment: %w", err)
	}
	return &p, nil
}

func (r *TravelRepository) GetPaymentsByOrder(ctx context.Context, orderID uuid.UUID) ([]model.Payment, error) {
	var payments []model.Payment
	query := "SELECT * FROM payments WHERE order_id = $1 ORDER BY created_at DESC"
	if err := r.db.SelectContext(ctx, &payments, r.db.Rebind(query), orderID); err != nil {
		return nil, fmt.Errorf("get payments by order: %w", err)
	}
	return payments, nil
}

func (r *TravelRepository) UpdatePaymentStatus(ctx context.Context, id uuid.UUID, status, transactionID string) error {
	query := "UPDATE payments SET status = $1, transaction_id = $2 WHERE id = $3"
	_, err := r.db.ExecContext(ctx, r.db.Rebind(query), status, transactionID, id)
	return err
}

func (r *TravelRepository) UpdateVoucher(ctx context.Context, v *model.Voucher) error {
	query := `
		UPDATE vouchers SET title = :title, description = :description, destination_id = :destination_id, price = :price, currency = :currency, duration_nights = :duration_nights, includes_flight = :includes_flight, includes_hotel = :includes_hotel, includes_meal = :includes_meal, image_url = :image_url, is_active = :is_active
		WHERE id = :id
	`
	_, err := r.db.NamedExecContext(ctx, query, v)
	return err
}

func (r *TravelRepository) DeleteVoucher(ctx context.Context, id uuid.UUID) error {
	query := "DELETE FROM vouchers WHERE id = $1"
	_, err := r.db.ExecContext(ctx, r.db.Rebind(query), id)
	return err
}

func (r *TravelRepository) UpdateDestination(ctx context.Context, d *model.Destination) error {
	query := `
		UPDATE destinations SET name = :name, country = :country, city = :city, description = :description, image_url = :image_url, rating = :rating, price_from = :price_from, currency = :currency, source = :source, external_id = :external_id, updated_at = NOW()
		WHERE id = :id
	`
	_, err := r.db.NamedExecContext(ctx, query, d)
	return err
}

func (r *TravelRepository) DeleteDestination(ctx context.Context, id uuid.UUID) error {
	query := "DELETE FROM destinations WHERE id = $1"
	_, err := r.db.ExecContext(ctx, r.db.Rebind(query), id)
	return err
}

func (r *TravelRepository) UpdateTour(ctx context.Context, t *model.Tour) error {
	query := `
		UPDATE tours SET destination_id = :destination_id, title = :title, description = :description, price = :price, currency = :currency, duration_days = :duration_days, start_date = :start_date, end_date = :end_date, available = :available, image_url = :image_url, source = :source, external_id = :external_id, updated_at = NOW()
		WHERE id = :id
	`
	_, err := r.db.NamedExecContext(ctx, query, t)
	return err
}

func (r *TravelRepository) DeleteTour(ctx context.Context, id uuid.UUID) error {
	query := "DELETE FROM tours WHERE id = $1"
	_, err := r.db.ExecContext(ctx, r.db.Rebind(query), id)
	return err
}

func (r *TravelRepository) ListUsers(ctx context.Context, p *model.Pagination) ([]model.User, int, error) {
	var users []model.User
	query := "SELECT * FROM users ORDER BY created_at DESC LIMIT $1 OFFSET $2"
	if err := r.db.SelectContext(ctx, &users, r.db.Rebind(query), p.Limit(), p.Offset()); err != nil {
		return nil, 0, fmt.Errorf("list users: %w", err)
	}
	var total int
	if err := r.db.GetContext(ctx, &total, "SELECT COUNT(*) FROM users"); err != nil {
		return nil, 0, fmt.Errorf("count users: %w", err)
	}
	return users, total, nil
}

func (r *TravelRepository) ListAllOrders(ctx context.Context, p *model.Pagination) ([]model.Order, int, error) {
	var orders []model.Order
	query := "SELECT * FROM orders ORDER BY created_at DESC LIMIT $1 OFFSET $2"
	if err := r.db.SelectContext(ctx, &orders, r.db.Rebind(query), p.Limit(), p.Offset()); err != nil {
		return nil, 0, fmt.Errorf("list orders: %w", err)
	}
	var total int
	if err := r.db.GetContext(ctx, &total, "SELECT COUNT(*) FROM orders"); err != nil {
		return nil, 0, fmt.Errorf("count orders: %w", err)
	}
	return orders, total, nil
}

func (r *TravelRepository) CountDestinations(ctx context.Context) (int, error) {
	var n int
	err := r.db.GetContext(ctx, &n, "SELECT COUNT(*) FROM destinations")
	return n, err
}

func (r *TravelRepository) CountTours(ctx context.Context) (int, error) {
	var n int
	err := r.db.GetContext(ctx, &n, "SELECT COUNT(*) FROM tours")
	return n, err
}

func (r *TravelRepository) CountUsers(ctx context.Context) (int, error) {
	var n int
	err := r.db.GetContext(ctx, &n, "SELECT COUNT(*) FROM users")
	return n, err
}

func (r *TravelRepository) CountOrders(ctx context.Context) (int, error) {
	var n int
	err := r.db.GetContext(ctx, &n, "SELECT COUNT(*) FROM orders")
	return n, err
}

func (r *TravelRepository) SumRevenue(ctx context.Context) (float64, error) {
	var sum float64
	err := r.db.GetContext(ctx, &sum, "SELECT COALESCE(SUM(total_price),0) FROM orders WHERE status IN ('paid','completed')")
	return sum, err
}

func (r *TravelRepository) GetSyncJobs(ctx context.Context, limit int) ([]model.SyncJob, error) {
	var jobs []model.SyncJob
	query := "SELECT * FROM sync_jobs ORDER BY created_at DESC LIMIT $1"
	if err := r.db.SelectContext(ctx, &jobs, r.db.Rebind(query), limit); err != nil {
		return nil, fmt.Errorf("get sync jobs: %w", err)
	}
	return jobs, nil
}

func (r *TravelRepository) DeleteReview(ctx context.Context, id uuid.UUID) error {
	query := "DELETE FROM reviews WHERE id = $1"
	_, err := r.db.ExecContext(ctx, r.db.Rebind(query), id)
	return err
}

func (r *TravelRepository) SearchDestinations(ctx context.Context, q, country, city string, minRating, maxPrice float64, p *model.Pagination) ([]model.Destination, int, error) {
	var destinations []model.Destination
	where := "WHERE 1=1"
	var args []interface{}
	argIdx := 1
	if q != "" {
		where += fmt.Sprintf(" AND (name ILIKE $%d OR country ILIKE $%d OR city ILIKE $%d OR description ILIKE $%d)", argIdx, argIdx, argIdx, argIdx)
		args = append(args, "%"+q+"%")
		argIdx++
	}
	if country != "" {
		where += fmt.Sprintf(" AND country = $%d", argIdx)
		args = append(args, country)
		argIdx++
	}
	if city != "" {
		where += fmt.Sprintf(" AND city = $%d", argIdx)
		args = append(args, city)
		argIdx++
	}
	if minRating > 0 {
		where += fmt.Sprintf(" AND rating >= $%d", argIdx)
		args = append(args, minRating)
		argIdx++
	}
	if maxPrice > 0 {
		where += fmt.Sprintf(" AND price_from <= $%d", argIdx)
		args = append(args, maxPrice)
		argIdx++
	}
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM destinations %s", where)
	var total int
	if err := r.db.GetContext(ctx, &total, r.db.Rebind(countQuery), args...); err != nil {
		return nil, 0, fmt.Errorf("count destinations: %w", err)
	}
	query := fmt.Sprintf("SELECT * FROM destinations %s ORDER BY rating DESC, created_at DESC LIMIT $%d OFFSET $%d", where, argIdx, argIdx+1)
	args = append(args, p.Limit(), p.Offset())
	if err := r.db.SelectContext(ctx, &destinations, r.db.Rebind(query), args...); err != nil {
		return nil, 0, fmt.Errorf("search destinations: %w", err)
	}
	return destinations, total, nil
}

func (r *TravelRepository) SearchTours(ctx context.Context, q string, destID *uuid.UUID, available *bool, minPrice, maxPrice float64, minDuration, maxDuration int, p *model.Pagination) ([]model.Tour, int, error) {
	var tours []model.Tour
	where := "WHERE 1=1"
	var args []interface{}
	argIdx := 1
	if q != "" {
		where += fmt.Sprintf(" AND (title ILIKE $%d OR description ILIKE $%d)", argIdx, argIdx)
		args = append(args, "%"+q+"%")
		argIdx++
	}
	if destID != nil {
		where += fmt.Sprintf(" AND destination_id = $%d", argIdx)
		args = append(args, *destID)
		argIdx++
	}
	if available != nil {
		where += fmt.Sprintf(" AND available = $%d", argIdx)
		args = append(args, *available)
		argIdx++
	}
	if minPrice > 0 {
		where += fmt.Sprintf(" AND price >= $%d", argIdx)
		args = append(args, minPrice)
		argIdx++
	}
	if maxPrice > 0 {
		where += fmt.Sprintf(" AND price <= $%d", argIdx)
		args = append(args, maxPrice)
		argIdx++
	}
	if minDuration > 0 {
		where += fmt.Sprintf(" AND duration_days >= $%d", argIdx)
		args = append(args, minDuration)
		argIdx++
	}
	if maxDuration > 0 {
		where += fmt.Sprintf(" AND duration_days <= $%d", argIdx)
		args = append(args, maxDuration)
		argIdx++
	}
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM tours %s", where)
	var total int
	if err := r.db.GetContext(ctx, &total, r.db.Rebind(countQuery), args...); err != nil {
		return nil, 0, fmt.Errorf("count tours: %w", err)
	}
	query := fmt.Sprintf("SELECT * FROM tours %s ORDER BY created_at DESC LIMIT $%d OFFSET $%d", where, argIdx, argIdx+1)
	args = append(args, p.Limit(), p.Offset())
	if err := r.db.SelectContext(ctx, &tours, r.db.Rebind(query), args...); err != nil {
		return nil, 0, fmt.Errorf("search tours: %w", err)
	}
	return tours, total, nil
}

var _ = time.Now



func (r *TravelRepository) DistinctCountries(ctx context.Context) ([]string, error) {
	var countries []string
	query := "SELECT DISTINCT country FROM destinations ORDER BY country"
	if err := r.db.SelectContext(ctx, &countries, query); err != nil {
		return nil, err
	}
	return countries, nil
}



func (r *TravelRepository) UpdateUserPassword(ctx context.Context, userID uuid.UUID, passwordHash string) error {
	query := "UPDATE users SET password_hash = $1, updated_at = NOW() WHERE id = $2"
	_, err := r.db.ExecContext(ctx, r.db.Rebind(query), passwordHash, userID)
	return err
}

