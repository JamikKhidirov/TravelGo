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

Защищённые эндпоинты принимают:

```http
Authorization: Bearer <token>
X-User-Id: <uuid>
```

Токен выдаётся при регистрации или логине:

```bash
curl -X POST http://localhost:8080/api/v1/auth/register \
  -H "Content-Type: application/json" \
  -d '{
    "email": "user@example.com",
    "password": "password123",
    "first_name": "John",
    "last_name": "Doe",
    "phone": "+1234567890"
  }'
```

```json
{
  "success": true,
  "data": {
    "user": { "id": "uuid", "email": "user@example.com", "role": "user" },
    "token": "travel-token-<uuid>-user-<timestamp>"
  }
}
```

Логин:

```bash
curl -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"user@example.com","password":"password123"}'
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
| POST | `/api/v1/auth/register` | Регистрация |
| POST | `/api/v1/auth/login` | Логин |
| GET | `/api/v1/auth/me` | Текущий пользователь |
| GET | `/api/v1/auth/profile` | Профиль |
| PUT | `/api/v1/auth/profile` | Обновить профиль |
| POST | `/api/v1/auth/change-password` | Сменить пароль |
| POST | `/api/v1/auth/refresh` | Обновить токен |

Смена пароля:

```bash
curl -X POST http://localhost:8080/api/v1/auth/change-password \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -H "X-User-Id: $USER_ID" \
  -d '{"old_password":"password123","new_password":"newpass456"}'
```

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
| GET | `/api/v1/orders` | Заказы пользователя |
| GET | `/api/v1/orders/{id}` | Заказ по ID |
| POST | `/api/v1/orders/{id}/cancel` | Отменить заказ |
| PUT | `/api/v1/orders/{id}/status` | Изменить статус |

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
