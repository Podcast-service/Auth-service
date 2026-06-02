# Auth Service

Микросервис аутентификации и авторизации на Go. Поддерживает JWT + refresh tokens, multi-device сессии, верификацию email и сброс пароля.

## Быстрый старт

### 1. Создать `.env` файл

Пример лежит в `.env.example`. Важно указать `ACCESS_TOKEN_SECRET` для подписи JWT.

### 2. Запустить через Docker Compose

```bash
docker compose up --build
```

### 3. Проверить что сервис запущен

```bash
curl http://31.130.132.89/auth/me/roles
# ожидаем: 401 Unauthorized (сервис работает, токен не передан)
```

---

## UI для мониторинга

| Сервис | URL | Логин / Пароль   |
|---|---|------------------|
| RabbitMQ Management | http://31.130.132.89:15672 | admin / password |
| Kafka UI | http://31.130.132.89:8090 | —                |
 
---

## API

Базовый URL: `http://31.130.132.89`

### Публичные эндпоинты

| Метод | Путь | Описание |
|---|---|---|
| `POST` | `/auth/register` | Регистрация |
| `POST` | `/auth/verify-email` | Верификация email |
| `POST` | `/auth/resend-verification` | Повторная отправка кода верификации |
| `POST` | `/auth/login` | Вход |
| `POST` | `/auth/refresh` | Обновление токенов |
| `POST` | `/auth/password-reset/request` | Запрос сброса пароля |
| `POST` | `/auth/password-reset/confirm` | Подтверждение сброса пароля |

### Защищённые эндпоинты (требуют `Authorization: Bearer <token>`)

| Метод | Путь | Описание |
|---|---|---|
| `POST` | `/auth/logout` | Выход с текущего устройства |
| `POST` | `/auth/logout_all` | Выход со всех устройств |
| `POST` | `/auth/password-change` | Смена пароля |
| `GET` | `/auth/devices` | Список активных устройств |
| `GET` | `/auth/me/roles` | Роли текущего пользователя |
| `POST` | `/auth/me/update-roles` | Добавить роль пользователю |

### Admin-эндпоинты (требуют `Authorization: Bearer <token>` с ролью `admin`)

| Метод | Путь | Описание |
|---|---|---|
| `GET` | `/auth/admin/users` | Список пользователей с ролями (фильтры + пагинация) |
| `GET` | `/auth/admin/users/{user_id}` | Пользователь по id с ролями |
| `POST` | `/auth/admin/users/{user_id}/roles` | Добавить роль пользователю |
| `DELETE` | `/auth/admin/users/{user_id}/roles/admin` | Снять роль `admin` с пользователя |

Подробное описание — раздел [Admin API](#admin-api).
 
---

## Примеры взаимодействия

### Регистрация

```bash
curl -X POST http://31.130.132.89/auth/register \
  -H "Content-Type: application/json" \
  -d '{"email":"user@example.com","password":"secret123","username":"testuser"}'
```

**Ответ `201`:**
```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "email": "user@example.com"
}
```

После регистрации происходит два события:

**RabbitMQ** — в очереди `email.queue` появится код верификации.
Открой http://31.130.132.89:15672 → Queues → email.queue → Get messages:
```json
{
  "type": "EMAIL_VERIFY",
  "email": "user@example.com",
  "verify_code": "453177"
}
```

**Kafka** — в топике `podcast.user.register` появится событие.
Открой http://31.130.132.89:8090 → Topics → podcast.user.register → Messages:
```json
{
  "user_id": "550e8400-e29b-41d4-a716-446655440000",
  "username": "testuser"
}
```

---

### Повторная отправка кода верификации (если код не пришёл или истёк)

Если код не получен или уже истёк — запроси новый. Старые коды при этом инвалидируются (хранятся только 3 последних):

```bash
curl -X POST http://31.130.132.89/auth/resend-verification \
  -H "Content-Type: application/json" \
  -d '{"email":"user@example.com"}'
```

**Ответ `200`:**
```json
{
  "message": "if this email exists and is not verified, a new verification email has been sent"
}
```

В RabbitMQ появится новое сообщение с новым кодом.

### Верификация email

Возьми актуальный код из RabbitMQ и подставь в запрос:

```bash
curl -X POST http://31.130.132.89/auth/verify-email \
  -H "Content-Type: application/json" \
  -d '{"email":"user@example.com","code":"<$VERIFY_CODE>"}'
```

**Ответ `200`** — после верификации сразу выдаются токены для автологина:
```json
{
  "access_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "refresh_token": "d7f3a1b2c4e5...",
  "expires_in": 1800
}
```

---

### Вход

```bash
curl -X POST http://31.130.132.89/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"user@example.com","password":"secret123","device_name":"My Laptop2"}'
```

**Ответ `200`:**
```json
{
  "access_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "refresh_token": "d7f3a1b2c4e5...",
  "expires_in": 1800
}
```

Сохрани токены для следующих шагов:
```bash
ACCESS_TOKEN="eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."
REFRESH_TOKEN="d7f3a1b2c4e5..."
```

**Если email не подтверждён код высылается автоматически — `403`:**
```json
{
  "error": "email_not_verified",
  "message": "Email не подтверждён. Код верификации отправлен на почту."
}
```

---

### Обновление токенов

```bash
curl -X POST http://31.130.132.89/auth/refresh \
  -H "Content-Type: application/json" \
  -d "{\"refresh_token\":\"$REFRESH_TOKEN\"}"
```

**Ответ `200`** — старый refresh_token инвалидируется, выдаётся новая пара:
```json
{
  "access_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "refresh_token": "f6e5d4c3b2a1...",
  "expires_in": 1800
}
```

---

### Получить роли

```bash
curl http://31.130.132.89/auth/me/roles \
  -H "Authorization: Bearer $ACCESS_TOKEN"
```

**Ответ `200`:**
```json
{
  "roles": ["user"]
}
```

---

### Добавить роль

```bash
curl -X POST http://31.130.132.89/auth/me/update-roles \
  -H "Authorization: Bearer $ACCESS_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"role_name":"admin"}'
```

**Ответ `200`** — возвращается новый access_token с обновлёнными ролями:
```json
{
  "access_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "expires_in": 1800
}
```

Проверь что роль добавилась (используй новый токен из ответа):
```bash
curl http://31.130.132.89/auth/me/roles \
  -H "Authorization: Bearer <новый_access_token>"
```

**Ответ `200`:**
```json
{
  "roles": ["user", "admin"]
}
```

---

### Список активных устройств

```bash
curl http://31.130.132.89/auth/devices \
  -H "Authorization: Bearer $ACCESS_TOKEN"
```

**Ответ `200`:**
```json
[
  {
    "device_name": "My Laptop",
    "ip_address": "172.20.0.1",
    "user_agent": "curl/7.88.1",
    "created_at": "2026-04-29T09:31:00Z",
    "last_used_at": "2026-04-29T09:31:00Z",
    "refresh_token_id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890"
  }
]
```

---

### Запрос сброса пароля

```bash
curl -X POST http://31.130.132.89/auth/password-reset/request \
  -H "Content-Type: application/json" \
  -d '{"email":"user@example.com"}'
```

**Ответ `200`:**
```json
{
  "message": "if this email exists, a reset link has been sent"
}
```

В RabbitMQ появится сообщение:
```json
{
  "type": "PASSWORD_RESET",
  "email": "user@example.com",
  "reset_code": "782341"
}
```

---

### Подтверждение сброса пароля

```bash
curl -X POST http://31.130.132.89/auth/password-reset/confirm \
  -H "Content-Type: application/json" \
  -d '{"email":"user@example.com","code":"<$RESET_CODE>","new_password":"newSecret456"}'
```

**Ответ `200`:**
```json
{
  "message": "password has been reset"
}
```

---

### Смена пароля

```bash
curl -X POST http://31.130.132.89/auth/password-change \
  -H "Authorization: Bearer $ACCESS_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"old_password":"secret123","new_password":"newSecret456"}'
```

**Ответ `200`** — пароль изменён, текущие сессии пользователя сохраняются:
```json
{
  "message": "password has been changed"
}
```

---

### Выход с текущего устройства

```bash
curl -X POST http://31.130.132.89/auth/logout \
  -H "Authorization: Bearer $ACCESS_TOKEN" \
  -H "Content-Type: application/json" \
  -d "{\"refresh_token\":\"$REFRESH_TOKEN\"}"
```

**Ответ `200`:**
```json
{
  "message": "logged out"
}
```

---

### Выход со всех устройств

```bash
curl -X POST http://31.130.132.89/auth/logout_all \
  -H "Authorization: Bearer $ACCESS_TOKEN"
```

**Ответ `200`:**
```json
{
  "message": "logged out from all devices"
}
```

---

## Admin API

Эндпоинты под `/auth/admin` доступны только пользователям с ролью `admin`.
Каждый запрос проходит два слоя:

1. `AuthMiddleware` — валидация JWT access-токена. Без токена или с невалидным/просроченным токеном → `401 Unauthorized`.
2. `RequireRole("admin")` — проверка, что в claims токена присутствует роль `admin`. Валидный токен без роли `admin` → `403 Forbidden`.

Идентичность и роли актора берутся **только** из подписанного JWT, а не из тела запроса или query-параметров.

Auth-service является source of truth для users, roles и authorization. Допустимые роли (strict lowercase): `user`, `author`, `admin`.

### Общие коды ошибок

| Код | Когда |
|---|---|
| `400 Bad Request` | Невалидный UUID, неизвестная роль, некорректные query-параметры |
| `401 Unauthorized` | Токен отсутствует, невалиден или просрочен |
| `403 Forbidden` | Токен валиден, но у пользователя нет роли `admin` |
| `404 Not Found` | Целевой пользователь не найден |
| `409 Conflict` | Попытка снять роль `admin` с последнего администратора |
| `500 Internal Server Error` | Непредвиденная ошибка сервера/БД |

Формат ошибки: `{"error": "<текст>"}`.

---

### 1. Список пользователей

```
GET /auth/admin/users
Authorization: Bearer <ADMIN_ACCESS_TOKEN>
```

**Query-параметры (все опциональны):**

| Параметр | Тип | По умолчанию | Описание |
|---|---|---|---|
| `q` | string | — | Поиск по email (подстрока, регистронезависимо) |
| `role` | string | — | Фильтр по роли: `user`, `author`, `admin` |
| `email_verified` | boolean | — | Фильтр по подтверждению email |
| `page` | integer | `0` | Номер страницы (с нуля) |
| `size` | integer | `20` | Размер страницы (макс. `100`) |
| `sort` | string | `DATE_DESC` | Сортировка: `DATE_DESC`, `DATE_ASC`, `EMAIL_ASC` |

```bash
curl "http://31.130.132.89/auth/admin/users?role=admin&page=0&size=20&sort=DATE_DESC" \
  -H "Authorization: Bearer $ADMIN_ACCESS_TOKEN"
```

**Ответ `200`:**
```json
{
  "items": [
    {
      "id": "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa",
      "email": "user@example.com",
      "email_verified": true,
      "roles": ["author", "user"],
      "created_at": "2026-05-01T10:00:00Z",
      "updated_at": "2026-05-01T10:00:00Z"
    }
  ],
  "page": 0,
  "size": 20,
  "total_elements": 1,
  "total_pages": 1
}
```

**Ошибки:** `400` (некорректные query-параметры), `401`, `403`.

---

### 2. Пользователь по id

```
GET /auth/admin/users/{user_id}
Authorization: Bearer <ADMIN_ACCESS_TOKEN>
```

```bash
curl http://31.130.132.89/auth/admin/users/aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa \
  -H "Authorization: Bearer $ADMIN_ACCESS_TOKEN"
```

**Ответ `200`:**
```json
{
  "id": "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa",
  "email": "user@example.com",
  "email_verified": true,
  "roles": ["author", "user"],
  "created_at": "2026-05-01T10:00:00Z",
  "updated_at": "2026-05-01T10:00:00Z"
}
```

**Ошибки:** `400` (невалидный UUID), `401`, `403`, `404`.

---

### 3. Добавить роль пользователю

```
POST /auth/admin/users/{user_id}/roles
Authorization: Bearer <ADMIN_ACCESS_TOKEN>
Content-Type: application/json
```

**Тело запроса** (`role_name` ∈ `user`, `author`, `admin`):
```json
{ "role_name": "admin" }
```

```bash
curl -X POST http://31.130.132.89/auth/admin/users/aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa/roles \
  -H "Authorization: Bearer $ADMIN_ACCESS_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"role_name":"admin"}'
```

**Ответ `200`** — `roles` отражает актуальное состояние из БД:
```json
{
  "user_id": "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa",
  "roles": ["admin", "author", "user"],
  "changed": true
}
```

Если роль уже была у пользователя — операция идемпотентна, `changed: false`:
```json
{
  "user_id": "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa",
  "roles": ["admin", "author", "user"],
  "changed": false
}
```

**Ошибки:** `400` (невалидный UUID, пустой/неизвестный `role_name`), `401`, `403`, `404`.

---

### 4. Снять роль `admin`

Этот эндпоинт снимает **только** роль `admin`. Роли `user` и `author` через него снять нельзя.

```
DELETE /auth/admin/users/{user_id}/roles/admin
Authorization: Bearer <ADMIN_ACCESS_TOKEN>
```

```bash
curl -X DELETE http://31.130.132.89/auth/admin/users/aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa/roles/admin \
  -H "Authorization: Bearer $ADMIN_ACCESS_TOKEN"
```

**Ответ `200`:**
```json
{
  "user_id": "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa",
  "roles": ["author", "user"],
  "changed": true
}
```

Если у пользователя не было роли `admin` — операция идемпотентна, `changed: false`:
```json
{
  "user_id": "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa",
  "roles": ["author", "user"],
  "changed": false
}
```

**Защита последнего администратора:** если целевой пользователь — единственный
администратор в системе, запрос отклоняется с `409 Conflict`
(`{"error":"cannot remove the last admin"}`), роль не снимается.

**Ошибки:** `400` (невалидный UUID), `401`, `403`, `404`, `409`.

---

## События

### RabbitMQ — очередь `email.queue`

Используется для отправки писем пользователям.
Просмотр: http://31.130.132.89:15672 → Queues → email.queue → Get messages

| Тип | Когда отправляется |
|---|---|
| `EMAIL_VERIFY` | При регистрации, повторной отправке кода и попытке входа без верификации |
| `PASSWORD_RESET` | При запросе сброса пароля |

**Верификация email:**
```json
{
  "type": "EMAIL_VERIFY",
  "email": "user@example.com",
  "verify_code": "453177"
}
```

**Сброс пароля:**
```json
{
  "type": "PASSWORD_RESET",
  "email": "user@example.com",
  "reset_code": "782341"
}
```

### Kafka — топик `podcast.user.register`

Используется для уведомления других сервисов о новых пользователях.
Просмотр: http://31.130.132.89:8090 → Topics → podcast.user.register → Messages

**Регистрация пользователя:**
```json
{
  "user_id": "550e8400-e29b-41d4-a716-446655440000",
  "username": "testuser"
}
```
 
---
