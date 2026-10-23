package transport

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"github.com/google/uuid"
	"go-travel/internal/model"
)

type FlightSource struct {
	rand *rand.Rand
}

type RailSource struct {
	rand *rand.Rand
}

func NewFlightSource() *FlightSource {
	return &FlightSource{rand: rand.New(rand.NewSource(time.Now().UnixNano() + 200))}
}

func NewRailSource() *RailSource {
	return &RailSource{rand: rand.New(rand.NewSource(time.Now().UnixNano() + 300))}
}

func (s *FlightSource) Name() string { return "flight_msp_server" }

func (s *FlightSource) FetchDestinations(ctx context.Context) ([]model.Destination, error) {
	return []model.Destination{}, nil
}

func (s *FlightSource) FetchTours(ctx context.Context) ([]model.Tour, error) {
	return []model.Tour{}, nil
}

func (s *FlightSource) FetchTickets(ctx context.Context) ([]model.Ticket, error) {
	routes := []struct {
		From, FromCountry, To, ToCountry string
	}{
		{"Moscow", "Russia", "Istanbul", "Turkey"},
		{"Moscow", "Russia", "Dubai", "UAE"},
		{"Saint Petersburg", "Russia", "Paris", "France"},
		{"London", "United Kingdom", "New York", "USA"},
		{"Berlin", "Germany", "Rome", "Italy"},
		{"Almaty", "Kazakhstan", "Dubai", "UAE"},
		{"Tashkent", "Uzbekistan", "Moscow", "Russia"},
		{"Baku", "Azerbaijan", "Istanbul", "Turkey"},
	}
	airlines := []string{"Aeroflot", "Turkish Airlines", "Emirates", "Lufthansa", "Air Astana", "Uzbekistan Airways"}
	now := time.Now()
	var tickets []model.Ticket
	for i := 0; i < 28; i++ {
		r := routes[i%len(routes)]
		departure := now.Add(time.Duration(3+i*2) * time.Hour)
		tickets = append(tickets, model.Ticket{
			ID:                 uuid.New(),
			Type:               "flight",
			Provider:           s.Name(),
			OriginCity:         r.From,
			OriginCountry:      r.FromCountry,
			DestinationCity:    r.To,
			DestinationCountry: r.ToCountry,
			DepartureAt:        departure,
			ArrivalAt:          departure.Add(time.Duration(2+i%8) * time.Hour),
			Price:              150 + s.rand.Float64()*1200,
			Currency:           "USD",
			Carrier:            airlines[s.rand.Intn(len(airlines))],
			Number:             fmt.Sprintf("FL%d", 500+i),
			Class:              []string{"economy", "business", "first"}[i%3],
			AvailableSeats:     3 + s.rand.Intn(90),
			Source:             s.Name(),
			ExternalID:         fmt.Sprintf("%s:ticket:%d", s.Name(), i),
		})
	}
	return tickets, nil
}

func (s *RailSource) Name() string { return "rail_msp_server" }

func (s *RailSource) FetchDestinations(ctx context.Context) ([]model.Destination, error) {
	return []model.Destination{}, nil
}

func (s *RailSource) FetchTours(ctx context.Context) ([]model.Tour, error) {
	return []model.Tour{}, nil
}

func (s *RailSource) FetchTickets(ctx context.Context) ([]model.Ticket, error) {
	routes := []struct {
		From, FromCountry, To, ToCountry string
	}{
		{"Moscow", "Russia", "Saint Petersburg", "Russia"},
		{"Moscow", "Russia", "Kazan", "Russia"},
		{"Moscow", "Russia", "Sochi", "Russia"},
		{"Kazan", "Russia", "Yekaterinburg", "Russia"},
		{"Paris", "France", "Berlin", "Germany"},
		{"Berlin", "Germany", "Amsterdam", "Netherlands"},
		{"Rome", "Italy", "Milan", "Italy"},
		{"Madrid", "Spain", "Barcelona", "Spain"},
	}
	operators := []string{"Russian Railways", "EuroRail", "Night Express", "HighSpeed Rail"}
	now := time.Now()
	var tickets []model.Ticket
	for i := 0; i < 24; i++ {
		r := routes[i%len(routes)]
		departure := now.Add(time.Duration(5+i*4) * time.Hour)
		tickets = append(tickets, model.Ticket{
			ID:                 uuid.New(),
			Type:               "train",
			Provider:           s.Name(),
			OriginCity:         r.From,
			OriginCountry:      r.FromCountry,
			DestinationCity:    r.To,
			DestinationCountry: r.ToCountry,
			DepartureAt:        departure,
			ArrivalAt:          departure.Add(time.Duration(3+i%15) * time.Hour),
			Price:              30 + s.rand.Float64()*350,
			Currency:           "USD",
			Carrier:            operators[s.rand.Intn(len(operators))],
			Number:             fmt.Sprintf("TR%d", 700+i),
			Class:              []string{"second", "first", "sleeper"}[i%3],
			AvailableSeats:     10 + s.rand.Intn(120),
			Source:             s.Name(),
			ExternalID:         fmt.Sprintf("%s:ticket:%d", s.Name(), i),
		})
	}
	return tickets, nil
}
