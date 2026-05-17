# ДЗ 6. Онлайн обновление ленты новостей

## Архитектура push-уведомлений ленты

```plain
   Клиент → POST /post/create
           ↓
       Kafka (event stream)
           ↓
    Feed Worker (консьюмер)
      ↙              ↘
 Обновление Redis    Отправка в RabbitMQ Topic Exchange `posts.feed`
    (кэш ленты)                      routing_key = UUID подписчика
           ↓                                       ↓
   Redis-клиент обновляет ленту             WebSocket-сервис
                                    (exclusive queues + dynamic bind)
                                                   ↓
                                 Клиент по WS получает push-уведомление
```

1. **REST → Kafka**: при POST `/post/create` создаём событие в Kafka.
2. **Feed Worker**: читает события из Kafka, записывает свежую ленту в Redis и готовит уведомления для RabbitMQ.
3. **RabbitMQ**: Topic Exchange `posts.feed`. Routing key = UUID подписчика.
4. **WebSocket-сервис**: каждый экземпляр создаёт свою exclusive очередь, привязывает её к routing key активных пользователей и ретранслирует сообщения по WebSocket.

### Масштабирование RabbitMQ

При необходимости масштабирования можно:

- Запустить **кластер из 3+ узлов** и включить **Quorum Queues** или **Streams**, чтобы обеспечить репликацию очередей и отказоустойчивость.
- Применить **rabbitmq_sharding** plugin для равномерного распределения сообщений между нодами.
- Настроить **Federation** или **Shovel** для работы с географически распределёнными дата-центрами.

### Масштабирование WebSocket-сервиса

Каждый WS-экземпляр работает независимо:

- Создаёт _exclusive_ очередь и динамически биндит её на ключи подключённых пользователей.
- Получает только нужные сообщения, без лишних broadcast-ов.
- При добавлении новых инстансов пропускная способность растёт линейно — добавили сервер → разделили нагрузку.


## Hybrid Push / Pull модель ленты (защита от «celebrity»-нагрузки)

### Проблема

При чистом **push** (fan-out on write) один пост пользователя с 1 000 000 подписчиков порождает:
- 1 000 000 записей в Redis (ZADD в ленту каждого подписчика)
- 1 000 000 сообщений в RabbitMQ
- 1 000 000 WebSocket push-уведомлений

Это создаёт всплеск нагрузки и потенциально «убивает» систему.

### Решение — гибридная модель

Система разделяет друзей на два класса по порогу подписчиков (по умолчанию **10 000**, настраивается через `CELEBRITY_THRESHOLD`):

| Класс | Определение | Модель |
|-------|-------------|--------|
| Обычный пользователь | `followers_count < threshold` | **Push** — пост размножается в ленты всех подписчиков |
| Celebrity | `followers_count >= threshold` | **Pull** — пост хранится только в персональном ZSET; подписчики читают его при запросе ленты |

### Поле `is_celebrity` в БД

В таблице `users` добавлены атомарно обновляемые колонки:
- `followers_count INT` — текущее число подписчиков
- `is_celebrity BOOLEAN` — признак, вычисляемый при каждом добавлении/удалении друга в транзакции

При `AddFriend` / `DeleteFriend` счётчик обновляется в той же транзакции, что и `INSERT/DELETE` в `friendship`. Флаг пересчитывается относительно порога. Если статус пользователя меняется (обычный → celebrity или наоборот), сервис логирует событие и валидирует кеш `CelebrityResolver`.

### CelebrityResolver — двухуровневый кеш

Чтобы минимизировать обращения к БД при проверке статуса автора поста, используется `CelebrityResolver`:
- **L1** — in-memory map с TTL 30 секунд (per-process)
- **L2** — Redis с TTL 5 минут
- **L3** — реплика PostgreSQL (fallback)

При смене статуса пользователя кеш инвалидируется.

### Push-путь: обычный пользователь

```
Клиент → POST /post/create
               ↓
       Kafka Event (EventPostCreated)
               ↓
       Feed Worker → IsCelebrity(authorID) = false
               ↓
       Redis: INSERT в ZSET каждого подписчика + автора
               ↓
       RabbitMQ: user.{subscriberID} для каждого подписчика
               ↓
       WebSocket: BroadcastToUser(uid)
```

### Pull-путь: celebrity

```
Клиент → POST /post/create
               ↓
       Kafka Event (EventPostCreated)
               ↓
       Feed Worker → IsCelebrity(authorID) = true
               ↓
       Redis: INSERT только в:
         - feeder:{authorID}  (своя лента)
         - celeb_posts:{authorID}  (персональный ZSET celebrity)
               ↓
       RabbitMQ: НЕТ push-уведомлений подписчикам
               ↓
       WebSocket: НЕТ уведомлений
```

Подписчики **не получают** real-time уведомление, но при следующем запросе ленты (`GET /post/feed`) сервис вызывает `FetchAndMergeFeed`, который:
1. Читает push-ленту из Redis (обычные друзья)
2. Читает `celeb_posts` из Redis для каждого celebrity-друга
3. Сливает оба списка по времени (merge sorted by `created_at DESC`)
4. Возвращает страницу с учётом `limit`/`offset`

### Чтение ленты (Feed)

```
GET /post/feed
       ↓
  Redis: base = feeder:{userID}
       ↓
  Redis: celebPosts = celeb_posts:{celebA} ∪ celeb_posts:{celebB} ...
       ↓
  mergeSortedByTime(base, celebPosts)
       ↓
  ответ
```

Если Redis пуст, используется fallback в БД: `FeedFromAuthors(authorIDs = [self] + nonCelebFriends + celebFriends)`. При этом в кеш записываются **только** не-celebrity посты, чтобы push-cache оставался чистым.

### Пересчёт флагов при старте

Если в окружении задано `RECALC_CELEBRITY_ON_START=true`, при старте приложения выполняется SQL-функция `recalc_celebrity_flags(threshold)`, которая пересчитывает `followers_count` и `is_celebrity` для всех пользователей. Полезно при первом деплое механизма или смене порога.

### Итог

- Обычный пользователь: классический push, мгновенные WS-уведомления, O(N) операций, где N — число подписчиков (в среднем десятки-сотни).
- Celebrity: O(1) при посте, подписчики получают посты через merge при чтении ленты. Никакого миллионного fan-out.


## Детальная инструкция по проверке работы

Ниже приведены шаги, которые помогут убедиться, что вся цепочка обновления ленты работает корректно.

### 1. Запуск сервисов

Соберите и запустите все контейнеры с приложением и зависимостями:
```bash
docker compose up -d --build
```
Проверьте статус:
```bash
docker compose ps
```

### 2. Регистрация и логин пользователей

Зарегистрируйте двух пользователей (userA и userB) и получите их JWT:
```bash
# Регистрация
curl -X POST http://localhost:8080/auth/register \
  -H "Content-Type: application/json" \
  -d '{"username":"userA","password":"pass"}'

curl -X POST http://localhost:8080/auth/register \
  -H "Content-Type: application/json" \
  -d '{"username":"userB","password":"pass"}'

# Логин и получение JWT
TOKEN_A=$(curl -s -X POST http://localhost:8080/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"userA","password":"pass"}' | jq -r .token)
TOKEN_B=$(curl -s -X POST http://localhost:8080/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"userB","password":"pass"}' | jq -r .token)
```

### 3. Подписка (friend add)

Пусть userB подпишется на userA:
```bash
USER_A_ID=$(echo "$TOKEN_A" | jq -R 'split(".") | .[1] | @base64d | fromjson | .sub')
curl -X POST http://localhost:8080/friend/add \
  -H "Authorization: Bearer $TOKEN_B" \
  -H "Content-Type: application/json" \
  -d '{"friend_id":"'$USER_A_ID'"}'
```

### 4. Открытие WebSocket-подключения

Подключитесь от userB к WS-эндпоинту:
```bash
wscat -c 'ws://localhost:8080/post/feed/posted?token=$TOKEN_B'
```

> **Важно:** Не добавляйте слово `Bearer` в параметр `token` – передаётся только сам JWT.

### 5. Создание нового поста

От имени userA создайте пост:
```bash
curl -X POST http://localhost:8080/post/create \
  -H "Authorization: Bearer $TOKEN_A" \
  -H "Content-Type: application/json" \
  -d '{"content":"Привет от A!"}'
```

### 6. Ожидаем уведомления в WebSocket

В окне `wscat` вы должны получить JSON-сообщение:
```json
{
  "type":"post_created",
  "user_id":"<UUID_userB>",
  "post":{
    "post_id":"...",
    "author_id":"<UUID_userA>",
    "content":"Привет от A!",
    "created_at":"..."
  }
}
```

### 7. Проверка RabbitMQ UI

Перейдите в админку RabbitMQ (http://localhost:15672, guest/guest):
- Во вкладке **Exchanges** выберите `posts.feed`.
- Смотрите метрики **Published** – при создании поста счётчик должен увеличиться.
- В списке **Bindings** должна появиться динамическая очередь для вашего userB (routing key = UUID).