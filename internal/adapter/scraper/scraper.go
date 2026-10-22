package adapter

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"go-travel/internal/model"
)

type ScraperConfig struct {
	BaseURL     string
	HTTPClient  *http.Client
}

type PublicScraper struct {
	cfg ScraperConfig
}

func NewPublicScraper(baseURL string) *PublicScraper {
	return &PublicScraper{
		cfg: ScraperConfig{
			BaseURL: baseURL,
			HTTPClient: &http.Client{Timeout: 15 * time.Second},
		},
	}
}

func (s *PublicScraper) Name() string {
	return "public_scraper"
}

func (s *PublicScraper) doGet(ctx context.Context, path string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.cfg.BaseURL+path, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "go-travel-bot/1.0")

	resp, err := s.cfg.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d", resp.StatusCode)
	}

	return io.ReadAll(resp.Body)
}

func (s *PublicScraper) FetchDestinations(ctx context.Context) ([]model.Destination, error) {
	return nil, fmt.Errorf("scraper not implemented for this endpoint")
}

func (s *PublicScraper) FetchTours(ctx context.Context) ([]model.Tour, error) {
	return nil, fmt.Errorf("scraper not implemented for this endpoint")
}

var _ interface {
	FetchDestinations(context.Context) ([]model.Destination, error)
	FetchTours(context.Context) ([]model.Tour, error)
} = (*PublicScraper)(nil)

func DecodeDestinations(data []byte) ([]model.Destination, error) {
	var destinations []model.Destination
	if err := json.Unmarshal(data, &destinations); err != nil {
		return nil, err
	}
	return destinations, nil
}

func DecodeTours(data []byte) ([]model.Tour, error) {
	var tours []model.Tour
	if err := json.Unmarshal(data, &tours); err != nil {
		return nil, err
	}
	return tours, nil
}
