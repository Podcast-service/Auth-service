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
curl http://localhost:8080/auth/me/roles
# ожидаем: 401 Unauthorized (сервис работает, токен не передан)
```

---

## UI для мониторинга

| Сервис | URL | Логин / Пароль |
|---|---|---|
| RabbitMQ Management | http://localhost:15672 | user / password |
| Kafka UI | http://localhost:8090 | — |
 
---

## API

Базовый URL: `http://localhost:8080`

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
| `GET` | `/auth/devices` | Список активных устройств |
| `GET` | `/auth/me/roles` | Роли текущего пользователя |
| `POST` | `/auth/me/update-roles` | Добавить роль пользователю |
 
---

## Примеры взаимодействия

### Регистрация

```bash
curl -X POST http://localhost:8080/auth/register \
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
Открой http://localhost:15672 → Queues → email.queue → Get messages:
```json
{
  "type": "EMAIL_VERIFY",
  "email": "user@example.com",
  "verify_code": "453177"
}
```

**Kafka** — в топике `podcast.user.register` появится событие.
Открой http://localhost:8090 → Topics → podcast.user.register → Messages:
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
curl -X POST http://localhost:8080/auth/resend-verification \
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
curl -X POST http://localhost:8080/auth/verify-email \
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
curl -X POST http://localhost:8080/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"user@example.com","password":"secret123","device_name":"My Laptop1"}'
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
curl -X POST http://localhost:8080/auth/refresh \
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
curl http://localhost:8080/auth/me/roles \
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
curl -X POST http://localhost:8080/auth/me/update-roles \
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
curl http://localhost:8080/auth/me/roles \
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
curl http://localhost:8080/auth/devices \
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
curl -X POST http://localhost:8080/auth/password-reset/request \
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
curl -X POST http://localhost:8080/auth/password-reset/confirm \
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

### Выход с текущего устройства

```bash
curl -X POST http://localhost:8080/auth/logout \
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
curl -X POST http://localhost:8080/auth/logout_all \
  -H "Authorization: Bearer $ACCESS_TOKEN"
```

**Ответ `200`:**
```json
{
  "message": "logged out from all devices"
}
```

---

## События

### RabbitMQ — очередь `email.queue`

Используется для отправки писем пользователям.
Просмотр: http://localhost:15672 → Queues → email.queue → Get messages

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
Просмотр: http://localhost:8090 → Topics → podcast.user.register → Messages

**Регистрация пользователя:**
```json
{
  "user_id": "550e8400-e29b-41d4-a716-446655440000",
  "username": "testuser"
}
```
 
---
