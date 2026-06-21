# Разделение монолита на сервисы

В рамках задания подсистема **диалогов** была вынесена из монолита в отдельный сервис, работающий на собственной БД (Citus). Монолит и сервис диалогов скрыты за общим API-Gateway (**Traefik**), который маршрутизирует запросы и централизованно проверяет JWT.

---

## Содержание

- [Архитектура](#архитектура)
- [Компоненты](#компоненты)
- [Протокол взаимодействия монолит ⇄ dialog-svc](#протокол-взаимодействия-монолит--dialog-svc)
- [Маршрутизация в Traefik и совместимость со старыми клиентами](#маршрутизация-в-traefik-и-совместимость-со-старыми-клиентами)
- [Сквозное логирование (X-Request-ID)](#сквозное-логирование-x-request-id)
- [Сервис диалогов как отдельный артефакт](#сервис-диалогов-как-отдельный-артефакт)
- [Запуск](#запуск)
- [Проверка вручную](#проверка-вручную)
- [Структура репозитория](#структура-репозитория)

---

## Архитектура

```
                              ┌──────────────────────────┐
                              │        Traefik           │
                              │      (API Gateway)       │
                              │   http://localhost:80    │
                              └──────────┬───────────────┘
                                         │
                ┌────────────────────────┼─────────────────────────────┐
                │                        │                             │
                │   /v2/dialog/**        │ всё остальное               │
                │   (+ ForwardAuth →     │ /auth/**, /post/**, /v1/... │
                │    app:/auth/verify)   │                             │
                ▼                        ▼                             │
        ┌───────────────┐         ┌───────────────┐                    │
        │  dialog-svc   │         │   monolith    │                    │
        │  (отдельный   │◀────────│  (это репо)   │── /dialog/**      ─┘
        │   контейнер,  │  REST   │   Go, :8080   │  (старые клиенты)
        │   из Docker   │  HTTP   └──┬────────────┘
        │   Hub)        │            │
        └──────┬────────┘            │
               │                     │
               ▼                     ▼
        ┌────────────────┐    ┌──────────────────────────────────┐
        │  Citus         │    │  PostgreSQL master + 2 slaves    │
        │  (coordinator  │    │  Redis, Kafka, RabbitMQ          │
        │   + 3 workers) │    └──────────────────────────────────┘
        └────────────────┘
```

## Компоненты

| Сервис | Назначение | Порт (host) |
|---|---|---|
| `traefik` | API Gateway, маршрутизация старая/новая версия модуля диалогов, ForwardAuth | `80` |
| `app` (монолит, этот репозиторий) | Auth, профили, посты, лента, фолловеры, **legacy /dialog**| внутр. `:8080` |
| `dialog_svc` | Новый сервис диалогов (отдельный репозиторий, образ с Docker Hub) | внутр. `:8080` |
| `postgres-master` + 2 slaves | БД монолита, потоковая репликация | `5432–5434` |
| `citus-coordinator` + 3 workers | Шардированное хранилище сообщений (`messages`, шард-ключ `conversation_id`, 32 шарда) | `5435–5438` |
| `redis` | Кэш / счётчики | `6379` |
| `kafka` + `zookeeper` | Шина событий ленты | `9092`, `2181` |
| `rabbitmq` | Очереди (WebSocket-уведомления) | `5672`, UI `15672` |
| `prometheus`, `grafana` | Метрики и дашборды | `9090`, `3000` |

---

## Протокол взаимодействия монолит ⇄ dialog-svc

Выбран **REST поверх HTTP** (JSON). Причины: однородность с остальным API монолита, лёгкость отладки (curl/Postman), отсутствие необходимости в стриминговой семантике.

### Эндпоинты dialog-svc (внутренний контракт)

| Метод | Путь | Тело / параметры | Назначение |
|---|---|---|---|
| `POST` | `/dialog/{toUserId}/send` | `{"text": "..."}` | Отправить сообщение |
| `GET`  | `/dialog/{toUserId}/list` | - | Получить переписку с пользователем |

### Обязательные заголовки между сервисами

| Заголовок | Кто проставляет | Зачем |
|---|---|---|
| `X-User-ID` | Монолит (proxy-обработчик) **или** Traefik (после ForwardAuth) | Идентификатор аутентифицированного пользователя (`sub` из JWT). Сам токен в dialog-svc не передаётся — он уже валидирован выше |
| `X-Request-ID` | FE / При отсутствии хедера с фронта - монолит | Сквозная трассировка запроса |
| `Content-Type: application/json` | Всегда | — |

dialog-svc **никогда не валидирует JWT самостоятельно**. Он доверяет `X-User-ID`, потому что физически недоступен снаружи — обращение к нему возможно только через Traefik (с ForwardAuth) либо изнутри Docker-сети через монолит.

### Контракт DTO (см. `internal/models`)

```jsonc
// SendMessageDTO (request body)
{ "text": "Привет!" }

// GetMessageDTO (response item)
{
  "from": "uuid",
  "to":   "uuid",
  "text": "Привет!",
  "created_at": "2025-06-21T08:30:00Z"
}
```

---

## Маршрутизация в Traefik и совместимость со старыми клиентами

Старые клиенты, которые продолжают ходить в монолит, не должны ничего заметить. Сделано так:

### Старые клиенты (без изменений)

```
client ──► Traefik ──► monolith (app:8080)
            (любой путь, в т.ч. /dialog/...)
```

Старые клиенты бьют в монолит как и раньше. Если они стучатся в legacy-маршруты диалогов (`/dialog/...` в монолите), монолит **проксирует** запрос в `dialog_svc` через внутреннюю Docker-сеть, добавив `X-User-ID` из JWT-контекста и `X-Request-ID` (см. `internal/handlers/dialog.go`). Сам интерфейс HTTP для клиента не изменился.

### v2 — новые клиенты (ходят напрямую через шлюз)

```
client ──► Traefik
              │   /v2/dialog/{id}/{send|list}
              │
              ├──► ForwardAuth → app:/auth/verify   (валидация JWT)
              │                  └─► возвращает X-User-ID
              │
              ├──► strip-v2 (отрезает префикс /v2)
              │
              └──► dialog_svc:8080/dialog/{id}/{send|list}
```

Все это собрано в `docker-compose.yml` метками Traefik:

```yaml
# на контейнере dialog_svc:
- "traefik.http.routers.dialogs.rule=Host(`localhost`) && PathPrefix(`/v2/dialog/`)"
- "traefik.http.routers.dialogs.priority=100"  # выше монолита
- "traefik.http.middlewares.monolith-auth.forwardauth.address=http://app:8080/auth/verify"
- "traefik.http.middlewares.monolith-auth.forwardauth.authResponseHeaders=X-User-ID"
- "traefik.http.middlewares.strip-v2.stripprefix.prefixes=/v2"
- "traefik.http.routers.dialogs.middlewares=monolith-auth,strip-v2"
```

**JWT валидируется в единой точке** — в монолите, на эндпоинте `/auth/verify` (см. `internal/handlers/auth.go::VerifyHandler`). Traefik вызывает его через `ForwardAuth`, получает `X-User-ID`, прокидывает дальше в `dialog-svc`. Это снимает с нового сервиса задачи по работе с JWT (и с секретом `JWT_SECRET`) и оставляет ему единственную ответственность — хранение/выдачу сообщений.

### Сводная таблица

| Путь снаружи | Цель | Авторизация | Где живёт |
|---|---|---|---|
| `/v1/dialog/{id}/send` (legacy) | `monolith → dialog_svc` | JWT валидируется в монолите, дальше идёт `X-User-ID` | в коде монолита (`DialogHandler`) |
| `/v2/dialog/{id}/send` (new) | `Traefik → dialog_svc` напрямую | JWT валидируется через ForwardAuth → `app:/auth/verify` | в `docker-compose.yml` (labels) |
| `/auth/**`, `/post/**`, ... | `Traefik → monolith` | внутри монолита | — |

---

## Сквозное логирование (X-Request-ID)

- Любой входящий запрос, у которого нет `X-Request-ID`, получает свежий UUID в первом же сервисе, который его обрабатывает (Traefik пробрасывает заголовок без изменений; монолит/прокси генерирует, если пусто — см. `extSvcCallSend` в `internal/handlers/dialog.go`).
- При проксировании монолит → dialog-svc заголовок **всегда** проставляется в исходящий запрос.
- В логах Go используется `log/slog`, в каждой записи рядом с событием выводится `REQUESTID`, что позволяет склеить трассу запроса между двумя сервисами по grep.
- `VerifyHandler` в монолите тоже возвращает `X-Request-ID` обратно в ответе — это удобно при отладке через Traefik.

---

## Сервис диалогов как отдельный артефакт

> **Важно:** код сервиса диалогов **больше не лежит в этом репозитории**. Он развивается независимо и поставляется как готовый Docker-образ.

- **Docker Hub:** [`mblkuta/dialog-svc:0.2.1`](https://hub.docker.com/r/mblkuta/dialog-svc)
- **Multi-arch:** образ собран для **`linux/amd64`** и **`linux/arm64`** (manifest list), поэтому одинаково запускается и на CI/x86-серверах, и локально на Apple Silicon (M1/M2/M3).
- **Зависимости:** только Citus-кластер (`citus-coordinator`), который поднимается тем же `docker-compose.yml`.
- **Конфигурация** — через переменные окружения (см. блок `dialog_svc` в `docker-compose.yml`):

  ```yaml
  CITUS_HOST: citus-coordinator
  CITUS_PORT: 5432
  CITUS_DB:   social_dialogs
  CITUS_USER: citus_user
  CITUS_PASSWORD: citus_pass
  SERVER_ADDR: :8080
  ```

Таблица `messages` шардируется по `conversation_id` (32 шарда), workers регистрируются однократным джобом `citus-setup`.

---

## Запуск

Требования: Docker + Docker Compose v2.

```bash
docker compose up -d --build
```

Первый запуск занимает 1–2 минуты: поднимаются мастер/слейвы Postgres, инициализируется Citus и регистрируются workers. Полезные команды:

```bash
# Статус
docker compose ps

# Логи только шлюза и сервисов диалогов
docker compose logs -f traefik app dialog_svc

# Полный рестарт
docker compose down -v && docker compose up -d --build
```

Точки доступа:

| URL | Что |
|---|---|
| `http://localhost/` | Монолит через Traefik |
| `http://localhost/v2/dialog/{toUserId}/send` | Новый API диалогов |

---

## Проверка вручную

```bash
# 1. Регистрация + логин
curl -s -X POST http://localhost/auth/register \
  -H 'Content-Type: application/json' \
  -d '{"username":"alice","password":"pass","first_name":"A","second_name":"A"}'

TOKEN=$(curl -s -X POST http://localhost/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"username":"alice","password":"pass"}' | jq -r .access_token)

# 2. Старый клиент (v1) — попадает в монолит, монолит проксирует
curl -s -X POST http://localhost/dialog/<USER_UUID>/send \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -H 'X-Request-ID: trace-legacy-1' \
  -d '{"text":"hello via v1"}'

# 3. Новый клиент (v2) — попадает в dialog-svc напрямую,
#    JWT валидируется через ForwardAuth в монолите
curl -s -X POST http://localhost/v2/dialog/<USER_UUID>/send \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -H 'X-Request-ID: trace-v2-1' \
  -d '{"text":"hello via v2"}'

# 4. Получить переписку
curl -s -H "Authorization: Bearer $TOKEN" \
  "http://localhost/v2/dialog/<USER_UUID>/list?limit=50&offset=0"
```

---

## Структура репозитория

```
.
├── docker-compose.yml          # Все сервисы, в т.ч. dialog_svc из DockerHub
├── Dockerfile                  # Сборка монолита
├── main.go
├── db/
│   ├── master/                 # postgresql.conf + pg_hba для мастера
│   ├── slave/                  # конфиги для реплик
│   └── citus/                  # init-скрипты схемы messages + pg_hba
├── internal/
│   ├── auth/                   # JWT (HS256) + извлечение user_id из контекста
│   ├── handlers/
│   │   ├── auth.go             # /auth/login, /auth/register, /auth/verify (ForwardAuth)
│   │   ├── dialog.go           # legacy /dialog/** → проксирование в dialog_svc
│   │   └── server.go           # роутинг монолита
│   ├── services/               # бизнес-логика монолита
│   ├── store/                  # доступ к БД (master + replicas pool)
│   ├── metrics/                # Prometheus middleware
│   └── ws/                     # WebSocket для апдейтов ленты
└── prometheus.yml
```

---

## Соответствие требованиям ДЗ

| Требование | Реализация |
|---|---|
| Подсистема диалогов вынесена в отдельный сервис | Сервис `dialog_svc` живёт в отдельном репозитории, поставляется как multi-arch Docker-образ `mblkuta/dialog-svc:0.2.1` |
| Описан протокол взаимодействия | REST/HTTP+JSON, заголовки `X-User-ID`, `X-Request-ID` — см. раздел [«Протокол…»](#протокол-взаимодействия-монолит--dialog-svc) |
| Поддержка старых клиентов | `/v1/...` и legacy `/dialog/...` обрабатываются монолитом, который сам ходит в dialog-svc |
| Новые клиенты ходят через новый API | `/v2/dialog/...` — Traefik напрямую в `dialog_svc`, JWT проверяется через ForwardAuth |
| Сквозное логирование | `X-Request-ID` пробрасывается через Traefik и монолит, логируется через `log/slog` |
| Запуск одной командой | `docker compose up -d --build` |
