package msp2

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"github.com/google/uuid"
	"go-travel/internal/model"
)

type MSP2Source struct {
	rand *rand.Rand
}

func NewMSP2Source() *MSP2Source {
	return &MSP2Source{rand: rand.New(rand.NewSource(time.Now().UnixNano() + 100))}
}

func (s *MSP2Source) Name() string {
	return "msp2_server"
}

func (s *MSP2Source) FetchDestinations(ctx context.Context) ([]model.Destination, error) {
	countries := []struct {
		Country string
		Cities  []string
	}{
		{Country: "ОАЭ", Cities: []string{"Дубай", "Абу-Даби", "Аль-Айн"}},
		{Country: "Египет", Cities: []string{"Хургада", "Шарм-Эль-Шейх", "Марса-Алам"}},
		{Country: "Мальдивы", Cities: []string{"Мале", "Хулумале", "Атолл Баа"}},
		{Country: "Индия", Cities: []string{"Гואה", "Дели", "Мумбаи"}},
		{Country: "Чехия", Cities: []string{"Прага", "Брно", "Карлови Вары"}},
		{Country: "Хорватия", Cities: []string{"Загреб", "Сплит", "Дубровник"}},
	}

	var destinations []model.Destination
	for _, c := range countries {
		for _, city := range c.Cities {
			destinations = append(destinations, model.Destination{
				ID:          uuid.New(),
				Name:        city,
				Country:     c.Country,
				City:        city,
				Description: fmt.Sprintf("Премиум направление %s от MSP-2", city),
				ImageURL:    fmt.Sprintf("https://picsum.photos/seed/msp2-%s-%s/800/600", c.Country, city),
				Rating:      4.0 + s.rand.Float64()*1.0,
				PriceFrom:   500 + s.rand.Float64()*3000,
				Currency:    "USD",
				Source:      s.Name(),
				ExternalID:  fmt.Sprintf("%s:%s:%s", s.Name(), c.Country, city),
			})
		}
	}
	return destinations, nil
}

func (s *MSP2Source) FetchTours(ctx context.Context) ([]model.Tour, error) {
	titles := []string{
		"Роскошный отдых в ОАЭ",
		"Погружение в Египте",
		"Мальдивские острова",
		"Экзотика Индии",
		"Европейские каникулы",
		"Адриатическое побережье",
	}

	var tours []model.Tour
	now := time.Now()
	for i := 0; i < 20; i++ {
		start := now.AddDate(0, 0, i+2)
		end := start.AddDate(0, 0, 5+i%7)
		tours = append(tours, model.Tour{
			ID:            uuid.New(),
			DestinationID: uuid.New(),
			Title:         titles[s.rand.Intn(len(titles))],
			Description:   "Эксклюзивный тур от партнера MSP-2",
			Price:         800 + s.rand.Float64()*4000,
			Currency:      "USD",
			DurationDays:  5 + s.rand.Intn(12),
			StartDate:     start,
			EndDate:       end,
			Available:     true,
			ImageURL:      fmt.Sprintf("https://picsum.photos/seed/msp2-tour-%d/800/600", i),
			Source:        s.Name(),
			ExternalID:    fmt.Sprintf("%s:tour:%d", s.Name(), i),
		})
	}
	return tours, nil
}

var _ interface {
	FetchDestinations(context.Context) ([]model.Destination, error)
	FetchTours(context.Context) ([]model.Tour, error)
} = (*MSP2Source)(nil)
