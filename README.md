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