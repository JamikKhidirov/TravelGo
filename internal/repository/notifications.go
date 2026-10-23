package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"go-travel/internal/model"
)

func (r *TravelRepository) CreateTicket(ctx context.Context, t *model.Ticket) error {
	query := `
		INSERT INTO tickets (id, type, provider, origin_city, origin_country, destination_city, destination_country,
			departure_at, arrival_at, price, currency, carrier, number, class, available_seats, source, external_id)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17)
		ON CONFLICT (source, external_id) DO UPDATE SET
			type = EXCLUDED.type,
			provider = EXCLUDED.provider,
			origin_city = EXCLUDED.origin_city,
			origin_country = EXCLUDED.origin_country,
			destination_city = EXCLUDED.destination_city,
			destination_country = EXCLUDED.destination_country,
			departure_at = EXCLUDED.departure_at,
			arrival_at = EXCLUDED.arrival_at,
			price = EXCLUDED.price,
			currency = EXCLUDED.currency,
			carrier = EXCLUDED.carrier,
			number = EXCLUDED.number,
			class = EXCLUDED.class,
			available_seats = EXCLUDED.available_seats,
			updated_at = NOW()
		RETURNING id, created_at, updated_at`
	row := r.db.QueryRowxContext(ctx, r.db.Rebind(query),
		t.ID, t.Type, t.Provider, t.OriginCity, t.OriginCountry, t.DestinationCity, t.DestinationCountry,
		t.DepartureAt, t.ArrivalAt, t.Price, t.Currency, t.Carrier, t.Number, t.Class, t.AvailableSeats, t.Source, t.ExternalID)
	return row.Scan(&t.ID, &t.CreatedAt, &t.UpdatedAt)
}

func (r *TravelRepository) GetTickets(ctx context.Context, typ, origin, destination string, from *time.Time, p *model.Pagination) ([]model.Ticket, int, error) {
	where := []string{"1=1"}
	args := []interface{}{}
	idx := 1
	if typ != "" {
		where = append(where, fmt.Sprintf("type = $%d", idx))
		args = append(args, typ)
		idx++
	}
	if origin != "" {
		where = append(where, fmt.Sprintf("LOWER(origin_city) LIKE $%d", idx))
		args = append(args, "%"+strings.ToLower(origin)+"%")
		idx++
	}
	if destination != "" {
		where = append(where, fmt.Sprintf("LOWER(destination_city) LIKE $%d", idx))
		args = append(args, "%"+strings.ToLower(destination)+"%")
		idx++
	}
	if from != nil {
		where = append(where, fmt.Sprintf("departure_at >= $%d", idx))
		args = append(args, *from)
		idx++
	}
	clause := strings.Join(where, " AND ")
	var total int
	if err := r.db.GetContext(ctx, &total, r.db.Rebind("SELECT COUNT(*) FROM tickets WHERE "+clause), args...); err != nil {
		return nil, 0, err
	}
	query := fmt.Sprintf(`SELECT * FROM tickets WHERE %s ORDER BY departure_at ASC LIMIT $%d OFFSET $%d`, clause, idx, idx+1)
	args = append(args, p.Limit(), p.Offset())
	var tickets []model.Ticket
	if err := r.db.SelectContext(ctx, &tickets, r.db.Rebind(query), args...); err != nil {
		return nil, 0, err
	}
	return tickets, total, nil
}

func (r *TravelRepository) SearchTickets(ctx context.Context, q, typ string, p *model.Pagination) ([]model.Ticket, int, error) {
	where := []string{"1=1"}
	args := []interface{}{}
	idx := 1
	if q != "" {
		where = append(where, fmt.Sprintf("(LOWER(origin_city) LIKE $%d OR LOWER(destination_city) LIKE $%d OR LOWER(carrier) LIKE $%d)", idx, idx, idx))
		args = append(args, "%"+strings.ToLower(q)+"%")
		idx++
	}
	if typ != "" {
		where = append(where, fmt.Sprintf("type = $%d", idx))
		args = append(args, typ)
		idx++
	}
	clause := strings.Join(where, " AND ")
	var total int
	if err := r.db.GetContext(ctx, &total, r.db.Rebind("SELECT COUNT(*) FROM tickets WHERE "+clause), args...); err != nil {
		return nil, 0, err
	}
	query := fmt.Sprintf(`SELECT * FROM tickets WHERE %s ORDER BY price ASC, departure_at ASC LIMIT $%d OFFSET $%d`, clause, idx, idx+1)
	args = append(args, p.Limit(), p.Offset())
	var tickets []model.Ticket
	if err := r.db.SelectContext(ctx, &tickets, r.db.Rebind(query), args...); err != nil {
		return nil, 0, err
	}
	return tickets, total, nil
}

func (r *TravelRepository) GetTicket(ctx context.Context, id uuid.UUID) (*model.Ticket, error) {
	var t model.Ticket
	if err := r.db.GetContext(ctx, &t, r.db.Rebind("SELECT * FROM tickets WHERE id = $1"), id); err != nil {
		return nil, err
	}
	return &t, nil
}

func (r *TravelRepository) GetTicketsByCity(ctx context.Context, city string, limit int) ([]model.Ticket, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	var tickets []model.Ticket
	query := `SELECT * FROM tickets
		WHERE LOWER(origin_city) = LOWER($1) OR LOWER(destination_city) = LOWER($1)
		ORDER BY departure_at ASC LIMIT $2`
	if err := r.db.SelectContext(ctx, &tickets, r.db.Rebind(query), city, limit); err != nil {
		return nil, err
	}
	return tickets, nil
}

func (r *TravelRepository) CountTickets(ctx context.Context) (int, error) {
	var n int
	err := r.db.GetContext(ctx, &n, "SELECT COUNT(*) FROM tickets")
	return n, err
}

func (r *TravelRepository) UpsertDeviceToken(ctx context.Context, d *model.DeviceToken) error {
	query := `
		INSERT INTO device_tokens (id, user_id, platform, token, app_version, is_active)
		VALUES ($1,$2,$3,$4,$5,TRUE)
		ON CONFLICT (token) DO UPDATE SET
			user_id = EXCLUDED.user_id,
			platform = EXCLUDED.platform,
			app_version = EXCLUDED.app_version,
			is_active = TRUE,
			updated_at = NOW()
		RETURNING id, created_at, updated_at`
	return r.db.QueryRowxContext(ctx, r.db.Rebind(query), d.ID, d.UserID, d.Platform, d.Token, d.AppVersion).
		Scan(&d.ID, &d.CreatedAt, &d.UpdatedAt)
}

func (r *TravelRepository) DeactivateDeviceToken(ctx context.Context, token string) error {
	_, err := r.db.ExecContext(ctx, r.db.Rebind("UPDATE device_tokens SET is_active = FALSE, updated_at = NOW() WHERE token = $1"), token)
	return err
}

func (r *TravelRepository) GetDeviceTokensByUser(ctx context.Context, userID uuid.UUID) ([]model.DeviceToken, error) {
	var tokens []model.DeviceToken
	query := "SELECT * FROM device_tokens WHERE user_id = $1 AND is_active = TRUE ORDER BY created_at DESC"
	if err := r.db.SelectContext(ctx, &tokens, r.db.Rebind(query), userID); err != nil {
		return nil, err
	}
	return tokens, nil
}

func (r *TravelRepository) GetActiveDeviceTokens(ctx context.Context, platform string) ([]model.DeviceToken, error) {
	var tokens []model.DeviceToken
	args := []interface{}{}
	where := "is_active = TRUE"
	if platform != "" {
		where += " AND platform = $1"
		args = append(args, platform)
	}
	if err := r.db.SelectContext(ctx, &tokens, r.db.Rebind("SELECT * FROM device_tokens WHERE "+where+" ORDER BY created_at DESC"), args...); err != nil {
		return nil, err
	}
	return tokens, nil
}

func (r *TravelRepository) CreatePushNotification(ctx context.Context, n *model.PushNotification) error {
	query := `
		INSERT INTO push_notifications (id, user_id, title, body, data, platform, status, sent_count, failed_count)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
		RETURNING created_at`
	var userID interface{}
	if n.UserID != uuid.Nil {
		userID = n.UserID
	}
	return r.db.QueryRowxContext(ctx, r.db.Rebind(query), n.ID, userID, n.Title, n.Body, n.Data, n.Platform, n.Status, n.SentCount, n.FailedCount).
		Scan(&n.CreatedAt)
}

func (r *TravelRepository) GetPushNotifications(ctx context.Context, limit int) ([]model.PushNotification, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	var items []model.PushNotification
	if err := r.db.SelectContext(ctx, &items, r.db.Rebind("SELECT * FROM push_notifications ORDER BY created_at DESC LIMIT $1"), limit); err != nil {
		return nil, err
	}
	return items, nil
}

func (r *TravelRepository) GetVouchersByCity(ctx context.Context, city string, limit int) ([]model.Voucher, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	var vouchers []model.Voucher
	query := `SELECT v.* FROM vouchers v
		JOIN destinations d ON d.id = v.destination_id
		WHERE LOWER(d.city) = LOWER($1) AND v.is_active = TRUE
		ORDER BY v.price ASC LIMIT $2`
	if err := r.db.SelectContext(ctx, &vouchers, r.db.Rebind(query), city, limit); err != nil {
		return nil, err
	}
	return vouchers, nil
}

func (r *TravelRepository) CountDevices(ctx context.Context) (int, error) {
	var n int
	err := r.db.GetContext(ctx, &n, "SELECT COUNT(*) FROM device_tokens WHERE is_active = TRUE")
	return n, err
}

func (r *TravelRepository) CountPushNotifications(ctx context.Context) (int, error) {
	var n int
	err := r.db.GetContext(ctx, &n, "SELECT COUNT(*) FROM push_notifications")
	return n, err
}
