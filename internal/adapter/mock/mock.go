package mock

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"time"

	"github.com/google/uuid"
	"go-travel/internal/model"
)

// Source — контракт любого источника данных о путешествиях.
// Реализуйте этот интерфейс, чтобы подключить новый MSP-сервер или публичный источник.
type Source interface {
	Name() string
	FetchDestinations(ctx context.Context) ([]model.Destination, error)
	FetchTours(ctx context.Context) ([]model.Tour, error)
}

// MockSource имитирует MSP-сервер (旅行-провайдера) без API-ключа.
// В реальном проекте здесь будут HTTP-запросы к партнёрским MSP-серверам,
// парсинг открытых каталогов 여행, RSS-лент и т.д.
type MockSource struct {
	baseURL string
	rand    *rand.Rand
}

// NewMockSource создаёт источник MSP с заглушками.
func NewMockSource(baseURL string) *MockSource {
	return &MockSource{
		baseURL: baseURL,
		rand:    rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// Name возвращает идентификатор источника.
func (m *MockSource) Name() string {
	return "mock_msp_server"
}

// FetchDestinations возвращает направления из мок-MSP.
func (m *MockSource) FetchDestinations(ctx context.Context) ([]model.Destination, error) {
	// citiesPerCountry — пары страна → города, которые отдаёт MSP-сервер.
	countries := []struct {
		Country string
		Cities  []string
	}{
		{Country: "Италия", Cities: []string{"Рим", "Милан", "Венеция", "Флоренция"}},
		{Country: "Франция", Cities: []string{"Париж", "Ницца", "Лион", "Марсель"}},
		{Country: "Япония", Cities: []string{"Токио", "Киото", "Осака", "Хиросима"}},
		{Country: "Турция", Cities: []string{"Анталия", "Стамбул", "Бодрум", "Кемер"}},
		{Country: "Тайланд", Cities: []string{"Бангкок", "Пхукет", "Чиангмай", "Патайя"}},
		{Country: "Греция", Cities: []string{"Афины", "Санторини", "Миконос", "Крит"}},
		{Country: "Испания", Cities: []string{"Барселона", "Мадрид", "Валенсия", "Севилья"}},
		{Country: "Египет", Cities: []string{"Хургада", "Шарм-Эль-Шейх", "Луксор"}},
		{Country: "ОАЭ", Cities: []string{"Дубай", "Абу-Даби", "Шарджа"}},
	}

	var destinations []model.Destination
	for _, c := range countries {
		for _, city := range c.Cities {
			destinations = append(destinations, model.Destination{
				ID:          uuid.New(),
				Name:        city,
				Country:     c.Country,
				City:        city,
				Description: fmt.Sprintf("Удивительное место %s в стране %s", city, c.Country),
				ImageURL:    fmt.Sprintf("https://picsum.photos/seed/%s-%s/800/600", c.Country, city),
				Rating:      3.5 + m.rand.Float64()*1.5,
				PriceFrom:   200 + m.rand.Float64()*2000,
				Currency:    "USD",
				Source:      m.Name(),
				ExternalID:  fmt.Sprintf("%s:%s:%s", m.Name(), c.Country, city),
			})
		}
	}
	return destinations, nil
}

// FetchTours возвращает туры из мок-MSP.
func (m *MockSource) FetchTours(ctx context.Context) ([]model.Tour, error) {
	titles := []string{
		"Экскурсия по старому городу",
		"Гастрономический тур",
		"Приключения в горах",
		"Пляжный отдых все включено",
		"Культурное наследие",
		"Романтический уик-энд",
		"Семейный отпуск",
		"Эко-тур",
	}

	var tours []model.Tour
	now := time.Now()
	for i := 0; i < 30; i++ {
		start := now.AddDate(0, 0, i+1)
		end := start.AddDate(0, 0, 3+i%5)
		tours = append(tours, model.Tour{
			ID:            uuid.New(),
			DestinationID: uuid.New(),
			Title:         titles[m.rand.Intn(len(titles))],
			Description:   "Захватывающий тур от нашего партнера MSP",
			Price:         300 + m.rand.Float64()*3000,
			Currency:      "USD",
			DurationDays:  3 + m.rand.Intn(10),
			StartDate:     start,
			EndDate:       end,
			Available:     true,
			ImageURL:      fmt.Sprintf("https://picsum.photos/seed/tour-%d/800/600", i),
			Source:        m.Name(),
			ExternalID:    fmt.Sprintf("%s:tour:%d", m.Name(), i),
		})
	}
	return tours, nil
}

// MockDestinationsToJSON сериализует направления в JSON (для отладки/экспорта).
func MockDestinationsToJSON(destinations []model.Destination) ([]byte, error) {
	return json.Marshal(destinations)
}

// MockToursToJSON сериализует туры в JSON (для отладки/экспорта).
func MockToursToJSON(tours []model.Tour) ([]byte, error) {
	return json.Marshal(tours)
}
