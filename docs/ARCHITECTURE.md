# Архитектура GoTravel

GoTravel — backend-прослойка между клиентом и несколькими travel MSP-серверами.

## Поток данных

```mermaid
flowchart LR
    subgraph MSP["MSP / парсеры"]
        M1["mock_msp_server"]
        M2["msp2_server"]
        M3["cityapi_free parser"]
        M4["flight_msp_server"]
        M5["rail_msp_server"]
    end

    SYNC["SyncService\nfetch + merge"] --> DB[("PostgreSQL")]
    M1 --> SYNC
    M2 --> SYNC
    M3 --> SYNC
    M4 --> SYNC
    M5 --> SYNC

    DB --> API["REST API /api/v1"]
    DB --> WS["WebSocket /ws"]
    DB --> PUSH["PushService\nFCM / APNs"]
    API --> CLIENT["Web / Mobile / Partners"]
    WS --> CLIENT
    PUSH --> IOS["iOS"]
    PUSH --> ANDROID["Android"]
    ADMIN["Admin panel /admin"] --> API
    ADMIN --> PUSH
```

## Слои

```mermaid
flowchart TB
    H["handlers\nGin REST + WebSocket"] --> S["services\nbusiness logic"]
    S --> R["repository\nsqlx + PostgreSQL"]
    S --> A["adapters\nMSP + public parser"]
    S --> P["push provider\nFCM / log"]
    R --> DB[("PostgreSQL")]
    A --> EXT["External MSP APIs"]
```

- `handler` — HTTP-контракты, валидация входа, коды ответов.
- `service` — сценарии: синхронизация, мерж, билеты, заказы, push.
- `repository` — SQL, upsert по `(source, external_id)`, пагинация.
- `adapter` — интеграции с MSP и публичными источниками.
- `model` — доменные структуры и JSON-контракты.

## Синхронизация

1. `POST /api/v1/sync` запускает обход всех источников.
2. Для каждого источника создаётся запись в `sync_jobs`.
3. Загружаются destinations, tours и tickets (если источник поддерживает).
4. Данные сохраняются через upsert, поэтому повторная синхронизация не создаёт дубликаты.
5. Туры привязываются к направлениям, билеты доступны для фильтрации по городу, типу и дате.
6. Клиент получает уже объединённые данные из одного API.

## Источники

| Источник | Назначение |
|---|---|
| `mock_msp_server` | демо-направления, туры, билеты |
| `msp2_server` | второй MSP: туры и билеты |
| `cityapi_free` | публичный парсер стран/столиц |
| `flight_msp_server` | авиабилеты |
| `rail_msp_server` | ЖД-билеты |

Новый источник подключается реализацией интерфейса:

```go
type Source interface {
    Name() string
    FetchDestinations(context.Context) ([]model.Destination, error)
    FetchTours(context.Context) ([]model.Tour, error)
}

type TicketSource interface {
    FetchTickets(context.Context) ([]model.Ticket, error)
}
```

Достаточно добавить источник в `service.NewSyncService(...)` в `cmd/server/main.go`.

## Push-уведомления

- Клиент регистрирует FCM/APNs токен через `POST /api/v1/devices/register`.
- Админ отправляет уведомление через `POST /api/v1/admin/push` или панель `/admin`.
- `PushService` выбирает получателей: конкретный `user_id` или все активные устройства.
- При наличии `FCM_SERVER_KEY` используется Firebase Cloud Messaging (Android и iOS).
- Без ключа включается `log`-провайдер для локальной разработки.

## Аутентификация и безопасность

```mermaid
flowchart LR
    C["Client"] -->|"POST /auth/login"| A["AuthService"]
    A -->|"access JWT + refresh"| C
    C -->|"Authorization: Bearer"| M["AuthMiddleware"]
    M -->|"проверка подписи и сессии"| R[("auth_sessions")]
    M --> H["Handlers"]
    H -->|"RBAC/ABAC"| S["Service"]
    S --> DB[("PostgreSQL")]
```

- **Access token** — JWT HS256, содержит `sub`, `sid`, `role`, `exp`; секрет задаётся
  `JWT_SECRET` (в Docker — переменная compose), по умолчанию dev-значение.
- **Refresh token** — непрозрачная строка, в БД хранится только SHA-256 хеш. При каждом
  обновлении токен ротируется, старая запись инвалидируется.
- **Сессии** (`auth_sessions`) хранят device name, platform, IP, user-agent, время
  логина, `last_seen_at`, срок жизни и `revoked_at`. Middleware проверяет, что сессия
  не отозвана; `last_seen_at` обновляется с троттлингом раз в минуту.
- **MFA** (`user_mfa`) — TOTP SHA1/6 цифр/30 секунд, совместим с Google Authenticator.
  Секрет выдаётся через `/auth/mfa/setup`, включается подтверждением кода.
- **Passkeys** (`passkey_credentials`) — WebAuthn challenge + `clientDataJSON` проверяются
  на сервере; подпись attestation/assertion проверяется платформой (документированное
  ограничение).
- **OAuth2/OIDC** (`oauth_codes`) — authorization code живёт 5 минут, одноразовый,
  PKCE только `S256`. `id_token` — HS256, `/oauth/jwks` возвращает пустой набор ключей.
- **RBAC** — роли `user`, `guide`, `partner`, `admin`; middleware отклоняет запросы без
  нужной роли (403).
- **ABAC** — гид ограничен `tours.owner_id`, партнёр — `orders.partner_id`; проверки
  выполняются и в списках, и при чтении/изменении конкретной записи.

## Пул подключений PostgreSQL

`schema`-миграции выполняются при старте, после чего пул настраивается:

| Параметр | По умолчанию | Env |
|---|---|---|
| `max_open_conns` | 25 | `DB_MAX_OPEN_CONNS` |
| `max_idle_conns` | 10 | `DB_MAX_IDLE_CONNS` |
| `conn_max_lifetime` | 5m | `DB_CONN_MAX_LIFETIME` |
| `conn_max_idle_time` | 2m | — |

Значения из `configs/config.yaml` переопределяются переменными окружения. При старте
в лог пишется `db pool configured` с фактическими параметрами — это удобно проверять
в `docker compose logs app`.

## Хранение геолокации

WebSocket-хаб хранит подключения и позиции в памяти процесса. Для нескольких инстансов приложения нужен общий pub/sub (Redis, NATS или Kafka).
