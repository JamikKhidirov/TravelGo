# GoTravel REST API

Базовый URL:

- локально: `http://localhost:8080`
- Docker: `http://localhost:8080`
-production: `https://your-domain.example`

Все ответы имеют общий конверт:

```json
{
  "success": true,
  "data": {},
  "meta": {},
  "error": ""
}
```

`error` возвращается только при ошибке, `meta` — для пагинации.

## Авторизация

Защищённые эндпоинты принимают JWT access token:

```http
Authorization: Bearer <access_token>
```

Для локальной разработки и админ-панели сохранён fallback через заголовки (отключайте в production):

```http
X-User-Id: <uuid>
X-User-Role: user|guide|partner|admin
```

Токен выдаётся при регистрации или логине вместе с refresh-токеном:

```bash
curl -X POST http://localhost:8080/api/v1/auth/register \
  -H "Content-Type: application/json" \
  -H "X-Device-Name: MacBook Pro" \
  -H "X-Device-Platform: web" \
  -d '{
    "email": "user@example.com",
    "password": "password123",
    "first_name": "John",
    "last_name": "Doe",
    "phone": "+1234567890",
    "role": "user"
  }'
```

```json
{
  "success": true,
  "data": {
    "user": { "id": "uuid", "email": "user@example.com", "role": "user" },
    "token": "<access_token>",
    "tokens": {
      "access_token": "<JWT HS256>",
      "refresh_token": "<opaque rotating token>",
      "id_token": "<OIDC ID token>",
      "token_type": "Bearer",
      "expires_in": 1800,
      "session_id": "uuid"
    }
  }
}
```

Логин:

```bash
curl -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -H "X-Device-Name: iPhone 15" \
  -H "X-Device-Platform: ios" \
  -d '{"email":"user@example.com","password":"password123"}'
```

Если у аккаунта включена MFA, логин возвращает `mfa_required: true` и `mfa_token`.
Завершите вход по TOTP-коду:

```bash
curl -X POST http://localhost:8080/api/v1/auth/login/mfa \
  -H "Content-Type: application/json" \
  -d '{"mfa_token":"<mfa_token>","code":"123456"}'
```

Refresh-токены одноразовые: каждый `/auth/refresh` возвращает новую пару, старый refresh
инвалидируется. Access-токен привязан к сессии (`sid`) и перестаёт работать после
revoke-сессии или `logout-all`.

### Роли и RBAC/ABAC

| Роль | Права |
|---|---|
| `user` | свои корзина, заказы, платежи, отзывы, ваучеры |
| `guide` | создаёт/редактирует/удаляет только свои туры (`tours.owner_id`), в списке видит только свои |
| `partner` | видит и меняет статус только назначенных ему заказов (`orders.partner_id`) |
| `admin` | полный доступ к `/api/v1/admin/*`, назначение партнёров на заказ |

Публичная регистрация разрешает только `user`, `guide`, `partner`; `admin` заводится
оператором (SQL/миграция), чтобы нельзя было получить админку через `/auth/register`.

Назначение партнёра на заказ (admin):

```bash
curl -X POST http://localhost:8080/api/v1/admin/orders/$ORDER_ID/partner \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -d '{"partner_id":"<partner-uuid>"}'
```

## Пагинация

Query-параметры:

| Параметр | Тип | По умолчанию | Ограничение |
|---|---|---|---|
| `page` | int | 1 | >= 1 |
| `page_size` | int | 20 | 1..100 |

Пример:

```bash
curl "http://localhost:8080/api/v1/destinations?page=1&page_size=20"
```

## Эндпоинты

### Health

| Метод | Путь | Описание |
|---|---|---|
| GET | `/health` | Проверка здоровья |
| GET | `/api/v1/health` | Проверка здоровья (versioned) |

### Destinations

| Метод | Путь | Описание |
|---|---|---|
| GET | `/api/v1/destinations` | Список направлений |
| GET | `/api/v1/destinations/search` | Поиск (`q`, `country`, `city`, `min_rating`, `max_price`) |
| GET | `/api/v1/destinations/top` | Топ направлений (`limit`) |
| GET | `/api/v1/destinations/countries` | Список стран |
| GET | `/api/v1/destinations/{id}` | Направление по ID |
| POST | `/api/v1/destinations` | Создать направление |
| PUT | `/api/v1/destinations/{id}` | Обновить направление |
| DELETE | `/api/v1/destinations/{id}` | Удалить направление |

Пример создания:

```bash
curl -X POST http://localhost:8080/api/v1/destinations \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Rome",
    "country": "Italy",
    "city": "Rome",
    "description": "Eternal city",
    "rating": 4.8,
    "price_from": 1200,
    "currency": "USD"
  }'
```

### Tours

| Метод | Путь | Описание |
|---|---|---|
| GET | `/api/v1/tours` | Список туров (`destination_id`, `available`) |
| GET | `/api/v1/tours/search` | Поиск (`q`, цены, длительность) |
| GET | `/api/v1/tours/featured` | Популярные туры (`limit`) |
| GET | `/api/v1/tours/{id}` | Тур по ID |
| POST | `/api/v1/tours` | Создать тур |
| PUT | `/api/v1/tours/{id}` | Обновить тур |
| DELETE | `/api/v1/tours/{id}` | Удалить тур |

```bash
curl -X POST http://localhost:8080/api/v1/tours \
  -H "Content-Type: application/json" \
  -d '{
    "destination_id": "<destination-uuid>",
    "title": "Rome Weekend",
    "description": "3 days in Rome",
    "price": 500,
    "currency": "USD",
    "duration_days": 3,
    "start_date": "2026-12-01T00:00:00Z",
    "end_date": "2026-12-04T00:00:00Z",
    "available": true
  }'
```

### Events и Sync

| Метод | Путь | Описание |
|---|---|---|
| GET | `/api/v1/events` | События (`aggregate`, `aggregate_id`, `limit`) |
| POST | `/api/v1/sync` | Синхронизация всех источников |
| GET | `/api/v1/sync/jobs` | История синхронизаций |

### Auth

| Метод | Путь | Описание |
|---|---|---|
| POST | `/api/v1/auth/register` | Регистрация (`user`, `guide`, `partner`) |
| POST | `/api/v1/auth/login` | Логин, возвращает токены и сессию |
| POST | `/api/v1/auth/login/mfa` | Завершить логин TOTP-кодом |
| POST | `/api/v1/auth/refresh` | Ротация refresh-токена |
| POST | `/api/v1/auth/logout` | Выйти на текущем устройстве |
| POST | `/api/v1/auth/logout-all` | Выйти на всех устройствах |
| GET | `/api/v1/auth/sessions` | Сессии, устройства, IP, время входа + сводка |
| GET | `/api/v1/auth/devices` | Алиас `/auth/sessions` |
| DELETE | `/api/v1/auth/sessions/{id}` | Отозвать конкретную сессию |
| GET | `/api/v1/auth/me` | Текущий пользователь и `session_id` |
| GET | `/api/v1/auth/profile` | Профиль |
| PUT | `/api/v1/auth/profile` | Обновить профиль |
| POST | `/api/v1/auth/change-password` | Сменить пароль |
| GET | `/api/v1/auth/mfa/status` | Статус MFA |
| POST | `/api/v1/auth/mfa/setup` | Начать подключение TOTP |
| POST | `/api/v1/auth/mfa/enable` | Подтвердить и включить TOTP |
| POST | `/api/v1/auth/mfa/disable` | Отключить TOTP |
| POST | `/api/v1/auth/passkeys/register/begin` | Начать регистрацию passkey |
| POST | `/api/v1/auth/passkeys/register/finish` | Завершить регистрацию passkey |
| GET | `/api/v1/auth/passkeys` | Список passkey |
| DELETE | `/api/v1/auth/passkeys/{id}` | Удалить passkey |
| POST | `/api/v1/auth/passkeys/login/begin` | Начать вход по passkey |
| POST | `/api/v1/auth/passkeys/login/finish` | Завершить вход по passkey |

Смена пароля:

```bash
curl -X POST http://localhost:8080/api/v1/auth/change-password \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"old_password":"password123","new_password":"newpass456"}'
```

### Сессии и устройства

`GET /auth/sessions` отдаёт список устройств и сводку:

```json
{
  "success": true,
  "data": {
    "sessions": [
      {
        "session_id": "uuid",
        "device": "iPhone 15",
        "platform": "ios",
        "ip": "203.0.113.10",
        "user_agent": "Mozilla/5.0 (iPhone; ...)",
        "login_at": "2026-10-27T12:00:07Z",
        "last_seen_at": "2026-10-27T12:05:11Z",
        "expires_at": "2026-11-26T12:00:07Z",
        "is_current": true,
        "is_active": true
      }
    ],
    "summary": {
      "total_sessions": 2,
      "active_sessions": 2,
      "platforms": { "ios": 1, "web": 1 },
      "unique_ips": ["203.0.113.10"],
      "last_login_at": "2026-10-27T12:00:07Z",
      "last_activity_at": "2026-10-27T12:05:11Z",
      "current_session_id": "uuid"
    }
  }
}
```

Название и платформа устройства берутся из заголовков `X-Device-Name` /
`X-Device-Platform`, иначе определяются из `User-Agent`. IP фиксируется на момент
логина. `last_seen_at` обновляется не чаще раза в минуту.

Выйти на всех устройствах:

```bash
curl -X POST http://localhost:8080/api/v1/auth/logout-all \
  -H "Authorization: Bearer $TOKEN"
```

### MFA (TOTP)

`POST /auth/mfa/setup` возвращает `secret` и `otpauth_url` — добавьте его в Google
Authenticator / 1Password / Aegis. Затем подтвердите кодом:

```bash
curl -X POST http://localhost:8080/api/v1/auth/mfa/enable \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"code":"123456"}'
```

Параметры: SHA1, 6 цифр, период 30 секунд, issuer `GoTravel`.

### Passkeys (WebAuthn)

Поток регистрации: `passkeys/register/begin` → нативный WebAuthn API → `passkeys/register/finish`.
Поток входа: `passkeys/login/begin` (email) → `passkeys/login/finish` → токены и новая сессия.
Сервер проверяет challenge, `clientDataJSON.type` и origin; подпись assertion
проверяется на клиенте/платформе — это задокументированное ограничение текущей реализации.

### OAuth2 / OIDC

| Метод | Путь | Описание |
|---|---|---|
| GET/POST | `/api/v1/oauth/authorize` | Выдача authorization code |
| POST | `/api/v1/oauth/token` | Обмен code/refresh на токены |
| GET/POST | `/api/v1/oauth/userinfo` | OIDC claims: `sub`, `email`, `name`, `role` |
| POST | `/api/v1/oauth/revoke` | Отзыв refresh-токена |
| GET | `/api/v1/oauth/jwks` | JWKS (HS256 — пустой набор ключей) |
| GET | `/api/v1/.well-known/openid-configuration` | OIDC discovery |

Мобильные клиенты используют Authorization Code + PKCE (`S256`):

```bash
# 1. code_challenge = base64url(sha256(verifier))
curl -X POST http://localhost:8080/api/v1/oauth/authorize \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{
    "client_id":"gotravel-mobile",
    "redirect_uri":"gotravel://callback",
    "response_type":"code",
    "scope":"openid profile email offline_access",
    "state":"xyz",
    "nonce":"nonce-123",
    "code_challenge":"<challenge>",
    "code_challenge_method":"S256"
  }'

# 2. Обмен кода
curl -X POST http://localhost:8080/api/v1/oauth/token \
  -H "Content-Type: application/json" \
  -d '{
    "grant_type":"authorization_code",
    "code":"<code>",
    "redirect_uri":"gotravel://callback",
    "client_id":"gotravel-mobile",
    "code_verifier":"<verifier>"
  }'
```

Код одноразовый и живёт 5 минут. `id_token` подписан HS256 тем же секретом, что и
access token, поэтому `/oauth/jwks` возвращает пустой набор ключей с пояснением.

### Cart

| Метод | Путь | Описание |
|---|---|---|
| GET | `/api/v1/cart` | Корзина пользователя |
| POST | `/api/v1/cart/add` | Добавить тур |
| PUT | `/api/v1/cart/item/{item_id}` | Изменить количество |
| DELETE | `/api/v1/cart/item/{item_id}` | Удалить позицию |
| POST | `/api/v1/cart/checkout` | Оформить заказ |
| POST | `/api/v1/cart/clear` | Очистить корзину |

```bash
curl -X POST http://localhost:8080/api/v1/cart/add \
  -H "Content-Type: application/json" \
  -H "X-User-Id: $USER_ID" \
  -d '{"tour_id":"<tour-uuid>","quantity":2,"price":500}'
```

### Orders

| Метод | Путь | Описание |
|---|---|---|
| GET | `/api/v1/orders` | Заказы пользователя; для `partner` — только назначенные ему |
| GET | `/api/v1/orders/{id}` | Заказ по ID (с проверкой владельца/партнёра) |
| POST | `/api/v1/orders/{id}/cancel` | Отменить заказ (владелец или admin) |
| PUT | `/api/v1/orders/{id}/status` | Изменить статус (партнёр — только свой заказ) |

### Payments

| Метод | Путь | Описание |
|---|---|---|
| POST | `/api/v1/payments` | Создать платёж |
| GET | `/api/v1/payments/{id}` | Платёж по ID |
| GET | `/api/v1/payments/order/{order_id}` | Платежи по заказу |
| POST | `/api/v1/payments/{id}/confirm` | Подтвердить платёж |

```bash
curl -X POST http://localhost:8080/api/v1/payments \
  -H "Content-Type: application/json" \
  -d '{"order_id":"<order-uuid>","amount":500,"currency":"USD","provider":"stripe"}'
```

### Reviews

| Метод | Путь | Описание |
|---|---|---|
| POST | `/api/v1/reviews` | Добавить отзыв |
| GET | `/api/v1/reviews/tour/{tour_id}` | Отзывы тура |
| DELETE | `/api/v1/reviews/{id}` | Удалить отзыв |

```bash
curl -X POST http://localhost:8080/api/v1/reviews \
  -H "Content-Type: application/json" \
  -H "X-User-Id: $USER_ID" \
  -d '{"tour_id":"<tour-uuid>","rating":5,"comment":"Great tour"}'
```

### Vouchers

| Метод | Путь | Описание |
|---|---|---|
| GET | `/api/v1/vouchers` | Список ваучеров |
| GET | `/api/v1/vouchers/{id}` | Ваучер по ID |
| POST | `/api/v1/vouchers` | Создать ваучер |
| PUT | `/api/v1/vouchers/{id}` | Обновить ваучер |
| DELETE | `/api/v1/vouchers/{id}` | Удалить ваучер |

### Tickets (flights, trains, buses)

Backend acts as an aggregation layer: tickets are collected from several MSP sources and public parsers, merged into PostgreSQL and returned to clients from one API.

| Метод | Путь | Описание |
|---|---|---|
| GET | `/api/v1/tickets` | Список билетов (`type`, `origin`, `destination`, `from`) |
| GET | `/api/v1/tickets/search` | Поиск (`q`, `type`) |
| GET | `/api/v1/tickets/{id}` | Билет по ID |
| GET | `/api/v1/cities/{city}/packages` | Пакеты по городу: destinations + tours + vouchers + tickets |

Примеры:

```bash
curl "http://localhost:8080/api/v1/tickets?type=flight&origin=Moscow&destination=Istanbul"
curl "http://localhost:8080/api/v1/tickets?type=train&page=1&page_size=20"
curl "http://localhost:8080/api/v1/tickets/search?q=Paris"
curl "http://localhost:8080/api/v1/cities/Moscow/packages"
```

Типы билетов: `flight`, `train`, `bus`.

### Devices и push-уведомления

Регистрация устройства iOS/Android (FCM/APNs token):

```bash
curl -X POST http://localhost:8080/api/v1/devices/register \
  -H "Content-Type: application/json" \
  -H "X-User-Id: $USER_ID" \
  -d '{"platform":"android","token":"fcm-token","app_version":"1.0.0"}'
```

| Метод | Путь | Описание |
|---|---|---|
| POST | `/api/v1/devices/register` | Зарегистрировать iOS/Android/Web токен |
| GET | `/api/v1/devices` | Устройства пользователя |
| DELETE | `/api/v1/devices/{token}` | Отключить устройство |
| POST | `/api/v1/admin/push` | Отправить push одному пользователю или всем |
| GET | `/api/v1/admin/push` | История push-уведомлений |
| GET | `/api/v1/admin/push/provider` | Активный провайдер (`fcm` или `log`) |

Отправка push:

```bash
curl -X POST http://localhost:8080/api/v1/admin/push \
  -H "Content-Type: application/json" \
  -d '{
    "user_id": "<uuid or empty for all>",
    "platform": "android",
    "title": "GoTravel",
    "body": "New tours available",
    "data": {"screen": "tours"}
  }'
```

Если задан `FCM_SERVER_KEY`, используется Firebase Cloud Messaging (Android и iOS). Без ключа работает `log`-провайдер — уведомления пишутся в лог, что удобно для разработки.

Админ-панель: [http://localhost:8080/admin](http://localhost:8080/admin)

### Admin

| Метод | Путь | Описание |
|---|---|---|
| GET | `/api/v1/admin/stats` | Статистика |
| GET | `/api/v1/admin/users` | Пользователи |
| GET | `/api/v1/admin/orders` | Все заказы |
| POST | `/api/v1/admin/orders/{id}/status` | Статус заказа |
| POST | `/api/v1/admin/orders/{id}/partner` | Назначить партнёра на заказ |
| POST | `/api/v1/admin/destinations` | Создать направление |
| POST | `/api/v1/admin/tours` | Создать тур |
| POST | `/api/v1/admin/vouchers` | Создать ваучер |
| POST | `/api/v1/admin/tickets` | Создать билет |
| POST | `/api/v1/admin/push` | Отправить push |
| POST | `/api/v1/admin/sync` | Запустить синхронизацию |
| GET | `/api/v1/admin/sync-jobs` | История синхронизаций |

## Swagger и Postman

- Swagger UI: [http://localhost:8080/swagger/index.html](http://localhost:8080/swagger/index.html)
- OpenAPI YAML: [api/swagger/swagger.yaml](../api/swagger/swagger.yaml)
- Postman collection: [postman/go-travel.postman_collection.json](../postman/go-travel.postman_collection.json)

## Коды ответов

| Код | Значение |
|---|---|
| 200 | OK |
| 201 | Created |
| 400 | Некорректные параметры |
| 401 | Неверные credentials |
| 403 | Недостаточно прав |
| 404 | Не найдено |
| 500 | Внутренняя ошибка |
