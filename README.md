![CI](https://github.com/anomalyco/GoTravel/actions/workflows/ci.yml/badge.svg)
![Go](https://img.shields.io/badge/Go-1.22-blue)
![Docker](https://img.shields.io/badge/Docker-ready-2496ED)

# GoTravel — Travel Microservice (Go)

Микросервис для сбора и выдачи данных о путешествиях на языке **Go**.  
Проект написан так, чтобы его мог поддерживать разработчик любой квалификации, в том числе не бэкенд-разработчик.

---

## Что делает сервис

1. **Собирает данные о путешествиях** из MSP-серверов и других открытых источников **без API-ключей**.
2. **Сохраняет всё в свою базу данных PostgreSQL**.
3. **Кэширует популярные запросы в Redis** (чтобы отвечать клиенту быстрее).
4. **Отправляет события об изменениях в Kafka** (туризм-лента, аудит, реалтайм).
5. **Отдаёт данные клиенту через REST API** (для веб-/мобилки).
6. **Передаёт обновления в реальном времени через WebSocket** (подходит для анимаций на клиенте).

---

## Откуда сервис собирает данные

| Источник | Что собирает | Нужен ли API-ключ |
|---|---|---|
| **Мок-MSP сервер** (`internal/adapter/mock`) | Направления (страны/города) и туры | ❌ Нет |
| **Публичные скраперы** (`internal/adapter/scraper`) | Открытые каталоги путешествий | ❌ Нет |
| **Своя PostgreSQL** | Уже собранные и сохранённые данные | — |

В коде каждый источник реализует интерфейс `Source` (файл `internal/adapter/mock/mock.go`).  
Чтобы добавить новый MSP-сервер, создайте новый адаптер и зарегистрируйте его в `cmd/server/main.go` — больше ничего менять не нужно.

---

## Архитектура проекта (простая схема)

```
Клиент (браузер / мобильное приложение / Postman)
        │
        ▼
┌─────────────────────┐
│  Gin REST API       │  /api/v1/destinations, /api/v1/tours,
│  + Swagger UI       │  /api/v1/events, /api/v1/sync, /health
│  + WebSocket (/ws)  │
└──────────┬──────────┘
           │
    ┌──────┴──────┐
    ▼             ▼
Handler      Service
(роуты,      (бизнес-логика,
 валидация)  события, кэш)
    │             │
    ▼             ▼
Repository   Cache (Redis)
(PostgreSQL) └─────────┘
    │
    ▼
Adapters (Mock MSP, Public Scraper)  ──► внешние MSP-серверы без API-ключей
    │
    ▼
Broker (Kafka) ──► события (создание направления, тур, синхронизация и т.д.)
```

Папки проекта:
- `cmd/server/main.go` — точка входа (запуск сервера).
- `internal/config/` — чтение конфигурации (`config.yaml` + переменные окружения).
- `internal/model/` — структуры данных (направления, туры, события, ответы API).
- `internal/repository/` — работа с PostgreSQL (запросы, миграции).
- `internal/service/` — бизнес-логика (создание списка, синхронизация с MSP).
- `internal/handler/` — REST-обработчики + WebSocket-хаб.
- `internal/cache/` — Redis-кэш.
- `internal/broker/` — Kafka-продюсер/консьюмер.
- `internal/adapter/` — источники данных (мок-MSP, скрапер, postgres).
- `api/swagger/swagger.yaml` — OpenAPI 3.0 спецификация.
- `docs/swagger/doc.go` — сгенерированная документация Swagger.
- `postman/go-travel.postman_collection.json` — коллекция для Postman.
- `configs/config.yaml` — основной конфиг.
- `docker-compose.yml` — инфраструктура (Postgres, Redis, Kafka, Zookeeper, приложение).

---

## Быстрый старт

### Что нужно установить
- **Docker** и **Docker Compose** (рекомендуемый способ).
- Либо вручную: **Go 1.22+**, PostgreSQL, Redis, Kafka, Zookeeper.

### Способ 1 — Docker (проще всего)

Из папки `go-travel` выполните:

```bash
docker-compose up --build
```

После этого:
- **API:** `http://localhost:8080`
- **Swagger UI:** `http://localhost:8080/swagger/index.html`
- **Health:** `http://localhost:8080/health`
- **WebSocket событий:** `ws://localhost:8080/ws/events`

Остановить:
```bash
docker-compose down
```

### Способ 2 — без Docker

```bash
cd go-travel
go mod tidy
go run ./cmd/server
```

Перед запуском отредактируйте `configs/config.yaml` под свои хосты/пароли.

---

## Переменные окружения (если не используете docker-compose)

| Переменная | По умолчанию | Описание |
|---|---|---|
| `APP_ENV` | `development` | Окружение (`development` / `production`) |
| `APP_PORT` | `8080` | Порт сервера |
| `DB_HOST` | `localhost` | Хост PostgreSQL |
| `DB_PORT` | `5432` | Порт PostgreSQL |
| `DB_USER` | `travel` | Пользователь БД |
| `DB_PASSWORD` | `travelpass` | Пароль БД |
| `DB_NAME` | `travel_db` | Имя базы данных |
| `REDIS_ADDR` | `localhost:6379` | Адрес Redis |
| `KAFKA_BROKERS` | `localhost:9092` | Брокеры Kafka через запятую |

---

## REST API (все endpoints)

| Метод | Путь | Описание |
|---|---|---|
| `GET` | `/health` | Проверка работы сервера |
| `GET` | `/api/v1/destinations` | Список направлений (`?country=`, `?city=`, `?page=`, `?page_size=`) |
| `GET` | `/api/v1/destinations/:id` | Одно направление по ID |
| `POST` | `/api/v1/destinations` | Создать направление (тело JSON) |
| `GET` | `/api/v1/tours` | Список туров (`?destination_id=`, `?available=`, `?page=`, `?page_size=`) |
| `GET` | `/api/v1/tours/:id` | Один тур по ID |
| `POST` | `/api/v1/tours` | Создать тур (тело JSON) |
| `GET` | `/api/v1/events` | Лента событий (`?aggregate=`, `?aggregate_id=`, `?limit=`) |
| `POST` | `/api/v1/sync` | Запустить сбор данных с MSP-серверов |
| `GET` | `/ws` | WebSocket (общий) |
| `GET` | `/ws/events` | WebSocket событий в реальном времени |

Swagger UI (интерактивная документация): `http://localhost:8080/swagger/index.html`

### Примеры curl

```bash
# Проверить, что сервер живой
curl http://localhost:8080/health

# Список направлений в Италии
curl "http://localhost:8080/api/v1/destinations?country=Италия&page_size=10"

# Создать направление
curl -X POST http://localhost:8080/api/v1/destinations \
  -H "Content-Type: application/json" \
  -d '{"name":"Рим","country":"Италия","city":"Рим","description":"Вечный город","rating":4.8,"price_from":1200,"currency":"USD","source":"manual","external_id":"manual:rome:1"}'

# Список туров
curl "http://localhost:8080/api/v1/tours?available=true&page_size=5"

# События (реальное время происходящего в базе)
curl "http://localhost:8080/api/v1/events?aggregate=destination&limit=20"

# Запустить синхронизацию с MSP-серверами
curl -X POST http://localhost:8080/api/v1/sync
```

---

## Postman

Импортируйте файл: `postman/go-travel.postman_collection.json`.

1. Откройте Postman → **Import** → выберите файл.
2. В переменных окружения задайте `baseUrl` = `http://localhost:8080`.
3. Отправляйте запросы из коллекции.

WebSocket `/ws/events` можно открыть в Postman (WebSocket-запрос) для получения событий в реальном времени.

---

## Swagger документация

- Спецификация: `api/swagger/swagger.yaml`.
- После запуска сервера откройте: `http://localhost:8080/swagger/index.html`.
- Там можно сразу пробовать все запросы без Postman.

---

## WebSocket / Реалтайм (для анимаций на клиенте)

Клиент может подключиться к `ws://localhost:8080/ws/events`.  
Когда в базе появляется новое направление, тур или происходит синхронизация, сервер рассылает событие всем подключённым клиентам.  
Это удобно для анимаций: например, на клиенте nuevos карточки направлений pojawiają się без перезагрузки страницы.

---

## Как добавить новый источник данных (MSP-сервер)

Если у вас появится ещё один MSP-сервер без API-ключа:

1. Создайте папку `internal/adapter/<имя_источника>/`.
2. Напишите структуру, реализующую интерфейс:

```go
type Source interface {
    Name() string
    FetchDestinations(ctx context.Context) ([]model.Destination, error)
    FetchTours(ctx context.Context) ([]model.Tour, error)
}
```

3. В `cmd/server/main.go` добавьте источник в `service.NewSyncService(repo, mockSrc, новыйИсточник)`.
4. Пересоберите: `docker-compose up --build`.

Готово — сервис начнёт собирать данные и из нового источника.

---

## Как поднять сервис на VPS (пошагово, для не бэкенд-разработчика)

### 1. Купите VPS
Рекомендуется: 2 vCPU, 4 GB RAM, Ubuntu 22.04 LTS.  
Подойдёт:any VPS (Timeweb, Yandex Cloud, Selectel, DigitalOcean, Hetzner и др.).

### 2. Подключитесь к серверу
```bash
ssh root@<IP_ВАШЕГО_VPS>
```

### 3. Установите Docker и Docker Compose
```bash
apt update && apt upgrade -y
apt install -y ca-certificates curl gnupg lsb-release
install -m 0755 -d /etc/apt/keyrings
curl -fsSL https://download.docker.com/linux/ubuntu/gpg | gpg --dearmor -o /etc/apt/keyrings/docker.gpg
echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.gpg] https://download.docker.com/linux/ubuntu $(lsb_release -cs) stable" | tee /etc/apt/sources.list.d/docker.list > /dev/null
apt update
apt install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin
systemctl enable --now docker
```

### 4. Склонируйте проект на VPS
```bash
apt install -y git
git clone <ссылка_на_ваш_репозиторий> /opt/go-travel
cd /opt/go-travel
```

### 5. Настройте конфиг (если нужно)
Отредактируйте `configs/config.yaml` — поменяйте пароли, если хотите. По умолчанию пароли уже прописаны в `docker-compose.yml`.

### 6. Запустите всё одной командой
```bash
docker-compose up -d --build
```

### 7. Проверьте, что работает
```bash
curl http://<IP_ВАШЕГО_VPS>:8080/health
```
Должен вернуться JSON `{"success":true,...}`.

Откройте в браузере: `http://<IP_ВАШЕГО_VPS>:8080/swagger/index.html` — там Swagger UI.

### 8. Откройте файрвол (если нужно)
Если на VPS стоит ufw:
```bash
ufw allow 8080/tcp
ufw allow 22/tcp
ufw enable
```

### 9. Остановка / перезапуск
```bash
cd /opt/go-travel
docker-compose down          # остановить
docker-compose up -d         # запустить снова
docker-compose logs -f app   # посмотреть логи приложения
```

### 10. (Опционально) Домен и SSL
Если у вас есть домен (`travel.example.com`):
- Установите Nginx или Traefik как обратный прокси на `8080`.
- Получите бесплатный SSL через Certbot (Let's Encrypt).
- Или используйте Cloudflare Tunnel — не нужно открывать порты и настраивать SSL вручную.

---

## Что добавлено «от себя» для полноценного бэкенда травел-приложения

- **Валидация запросов** (`go-playground/validator`) — не пустые обязательные поля, цена > 0.
- **Пагинация** — `page` и `page_size` на списках направлений и туров.
- **Логи в JSON** (`slog`) — удобно смотреть в Docker и на VPS.
- **Graceful shutdown** — сервер корректно завершается при `Ctrl+C`.
- **Health check** — `/health` для мониторинга и балансировщика нагрузки.
- **Event sourcing** — таблица `travel_events` хранит историю изменений (кто, когда, что создал).
- **Sync jobs** — таблица `sync_jobs` показывает статус сбора данных из MSP (в progress / completed / failed).
- **JWT-ready** — библиотека `golang-jwt/jwt/v5` уже в зависимостях, легко добавить авторизацию.
- **CORS-ready WebSocket** — можно подключать клиент с любого домена.

---

## Лицензия
MIT


---

## WebSocket Geolocation

Connect to `ws://localhost:8080/ws?room=global&user_id=<uuid>&name=Traveler`.

Client messages:
```json
{"type":"location","location":{"lat":55.7558,"lng":37.6173,"accuracy":12}}
{"type":"join","room":"room-1"}
{"type":"chat","message":"hello"}
```

Server broadcasts:
```json
{"type":"snapshot","room":"global","payload":[{"id":"...","user_id":"...","name":"Traveler","location":{"lat":0,"lng":0}}]}
{"type":"presence","room":"global","payload":[...]}
{"type":"location","room":"global","user_id":"...","name":"Traveler","location":{"lat":55.75,"lng":37.62}}
```

All connected clients in the same room receive location updates, enabling live map of travelers.

## Authentication

Use `Authorization: Bearer <token>` and `X-User-Id: <uuid>` headers for protected endpoints (cart, orders, reviews, profile).

## Data Sources

Sync collects from:
- `mock_msp_server`
- `msp2_server`
- `cityapi_free` (public CityAPI, no key)

Trigger with `POST /api/v1/sync`.

