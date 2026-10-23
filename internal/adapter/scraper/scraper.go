package adapter

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/google/uuid"
	"go-travel/internal/model"
)

type CityAPISource struct {
	baseURL string
	client  *http.Client
}

func NewCityAPISource() *CityAPISource {
	return &CityAPISource{
		baseURL: "https://api.cityapi.org",
		client:  &http.Client{Timeout: 20 * time.Second},
	}
}

func (s *CityAPISource) Name() string {
	return "cityapi_free"
}

func (s *CityAPISource) doGet(ctx context.Context, path string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.baseURL+path, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "go-travel-bot/1.0")
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d", resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}

type cityAPICountry struct {
	ISOAlpha2   string  `json:"iso_alpha2"`
	CommonName  string  `json:"common_name"`
	Capital     string  `json:"capital"`
	Region      string  `json:"region"`
	Subregion   string  `json:"subregion"`
	Population  int64   `json:"population"`
	Latitude    float64 `json:"latitude"`
	Longitude   float64 `json:"longitude"`
	FlagURL     string  `json:"flag_url"`
}

type cityAPICountriesResp struct {
	Data []cityAPICountry `json:"data"`
	Meta struct {
		HasMore bool `json:"has_more"`
	} `json:"meta"`
}

func (s *CityAPISource) FetchDestinations(ctx context.Context) ([]model.Destination, error) {
	data, err := s.doGet(ctx, "/v1/countries")
	if err != nil {
		return nil, err
	}
	var resp cityAPICountriesResp
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}
	var dests []model.Destination
	for _, c := range resp.Data {
		if c.Capital == "" {
			continue
		}
		price := 300.0 + float64(c.Population)/100000.0
		if price > 5000 {
			price = 5000
		}
		rating := 3.8 + (float64(len(c.CommonName)%5) * 0.1)
		if rating > 5 {
			rating = 5
		}
		dests = append(dests, model.Destination{
			ID:          uuid.New(),
			Name:        c.Capital,
			Country:     c.CommonName,
			City:        c.Capital,
			Description: fmt.Sprintf("Destination in %s (%s). Population: %d", c.CommonName, c.Region, c.Population),
			ImageURL:    "https://api.cityapi.org" + c.FlagURL,
			Rating:      rating,
			PriceFrom:   price,
			Currency:    "USD",
			Source:      s.Name(),
			ExternalID:  fmt.Sprintf("%s:country:%s", s.Name(), c.ISOAlpha2),
		})
	}
	return dests, nil
}

func (s *CityAPISource) FetchTours(ctx context.Context) ([]model.Tour, error) {
	data, err := s.doGet(ctx, "/v1/countries")
	if err != nil {
		return nil, err
	}
	var resp cityAPICountriesResp
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}
	now := time.Now()
	var tours []model.Tour
	titles := []string{
		"City Highlights Tour",
		"Cultural Discovery Package",
		"Adventure and Nature Trip",
		"Family Vacation Package",
		"Weekend Getaway",
	}
	for i, c := range resp.Data {
		if c.Capital == "" {
			continue
		}
		start := now.AddDate(0, 0, i+3)
		end := start.AddDate(0, 0, 4+i%6)
		tours = append(tours, model.Tour{
			ID:            uuid.New(),
			DestinationID: uuid.Nil,
			Title:         fmt.Sprintf("%s - %s", titles[i%len(titles)], c.Capital),
			Description:   fmt.Sprintf("Explore %s in %s with this curated travel package.", c.Capital, c.CommonName),
			Price:         400 + float64(i*37%2000),
			Currency:      "USD",
			DurationDays:  3 + i%8,
			StartDate:     start,
			EndDate:       end,
			Available:     true,
			ImageURL:      "https://api.cityapi.org" + c.FlagURL,
			Source:        s.Name(),
			ExternalID:    fmt.Sprintf("%s:tour:%s", s.Name(), c.ISOAlpha2),
		})
	}
	return tours, nil
}

var _ interface {
	FetchDestinations(context.Context) ([]model.Destination, error)
	FetchTours(context.Context) ([]model.Tour, error)
} = (*CityAPISource)(nil)
// keep Source alias
type Source = interface {
	Name() string
	FetchDestinations(context.Context) ([]model.Destination, error)
	FetchTours(context.Context) ([]model.Tour, error)
}
