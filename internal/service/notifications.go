package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/google/uuid"
	"go-travel/internal/model"
	"go-travel/internal/repository"
)

type TicketService struct {
	repo *repository.TravelRepository
}

func NewTicketService(repo *repository.TravelRepository) *TicketService {
	return &TicketService{repo: repo}
}

func (s *TicketService) List(ctx context.Context, typ, origin, destination string, from *time.Time, p *model.Pagination) ([]model.Ticket, int, error) {
	return s.repo.GetTickets(ctx, typ, origin, destination, from, p)
}

func (s *TicketService) Search(ctx context.Context, q, typ string, p *model.Pagination) ([]model.Ticket, int, error) {
	return s.repo.SearchTickets(ctx, q, typ, p)
}

func (s *TicketService) Get(ctx context.Context, id uuid.UUID) (*model.Ticket, error) {
	return s.repo.GetTicket(ctx, id)
}

func (s *TicketService) ByCity(ctx context.Context, city string, limit int) ([]model.Ticket, error) {
	return s.repo.GetTicketsByCity(ctx, city, limit)
}

func (s *TicketService) Create(ctx context.Context, t *model.Ticket) error {
	if t.ID == uuid.Nil {
		t.ID = uuid.New()
	}
	if t.Currency == "" {
		t.Currency = "USD"
	}
	if t.Source == "" {
		t.Source = "manual"
	}
	if t.ExternalID == "" {
		t.ExternalID = t.ID.String()
	}
	return s.repo.CreateTicket(ctx, t)
}

type PushProvider interface {
	Name() string
	Send(ctx context.Context, token, platform, title, body string, data map[string]string) error
}

type LogPushProvider struct{}

func (p *LogPushProvider) Name() string { return "log" }

func (p *LogPushProvider) Send(ctx context.Context, token, platform, title, body string, data map[string]string) error {
	log.Printf("[push:%s] token=%s title=%q body=%q data=%v", platform, maskToken(token), title, body, data)
	return nil
}

type FCMPushProvider struct {
	serverKey string
	client    *http.Client
}

func NewFCMPushProvider(serverKey string) *FCMPushProvider {
	return &FCMPushProvider{serverKey: serverKey, client: &http.Client{Timeout: 15 * time.Second}}
}

func (p *FCMPushProvider) Name() string { return "fcm" }

func (p *FCMPushProvider) Send(ctx context.Context, token, platform, title, body string, data map[string]string) error {
	payload := map[string]interface{}{
		"to":           token,
		"priority":     "high",
		"notification": map[string]string{"title": title, "body": body},
		"data":         data,
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://fcm.googleapis.com/fcm/send", bytes.NewReader(raw))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "key="+p.serverKey)
	resp, err := p.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		msg, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("fcm status %d: %s", resp.StatusCode, string(msg))
	}
	return nil
}

type PushService struct {
	repo     *repository.TravelRepository
	provider PushProvider
}

func NewPushService(repo *repository.TravelRepository) *PushService {
	var provider PushProvider = &LogPushProvider{}
	if key := os.Getenv("FCM_SERVER_KEY"); key != "" {
		provider = NewFCMPushProvider(key)
	}
	return &PushService{repo: repo, provider: provider}
}

func (s *PushService) Provider() string { return s.provider.Name() }

func (s *PushService) RegisterDevice(ctx context.Context, d *model.DeviceToken) error {
	if d.ID == uuid.Nil {
		d.ID = uuid.New()
	}
	return s.repo.UpsertDeviceToken(ctx, d)
}

func (s *PushService) UnregisterDevice(ctx context.Context, token string) error {
	return s.repo.DeactivateDeviceToken(ctx, token)
}

func (s *PushService) Devices(ctx context.Context, userID uuid.UUID) ([]model.DeviceToken, error) {
	return s.repo.GetDeviceTokensByUser(ctx, userID)
}

func (s *PushService) Send(ctx context.Context, userID *uuid.UUID, platform, title, body string, data map[string]string) (*model.PushNotification, error) {
	if title == "" || body == "" {
		return nil, fmt.Errorf("title and body are required")
	}
	var tokens []model.DeviceToken
	var err error
	if userID != nil && *userID != uuid.Nil {
		tokens, err = s.repo.GetDeviceTokensByUser(ctx, *userID)
	} else {
		tokens, err = s.repo.GetActiveDeviceTokens(ctx, platform)
	}
	if err != nil {
		return nil, err
	}

	rawData, _ := json.Marshal(data)
	n := &model.PushNotification{
		ID:       uuid.New(),
		Title:    title,
		Body:     body,
		Data:     string(rawData),
		Platform: platform,
		Status:   "running",
	}
	if userID != nil {
		n.UserID = *userID
	}

	for _, t := range tokens {
		if err := s.provider.Send(ctx, t.Token, t.Platform, title, body, data); err != nil {
			n.FailedCount++
			continue
		}
		n.SentCount++
	}
	if n.FailedCount > 0 && n.SentCount == 0 {
		n.Status = "failed"
	} else {
		n.Status = "completed"
	}
	if err := s.repo.CreatePushNotification(ctx, n); err != nil {
		return nil, err
	}
	return n, nil
}

func (s *PushService) History(ctx context.Context, limit int) ([]model.PushNotification, error) {
	return s.repo.GetPushNotifications(ctx, limit)
}

func maskToken(token string) string {
	if len(token) <= 8 {
		return "***"
	}
	return token[:4] + "..." + token[len(token)-4:]
}

func (s *VoucherService) ByCity(ctx context.Context, city string, limit int) ([]model.Voucher, error) {
	return s.repo.GetVouchersByCity(ctx, city, limit)
}
