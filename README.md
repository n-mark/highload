# Домашнее задание: In-Memory СУБД (Tarantool) для модуля диалогов

## Цель

Перенос хранения модуля диалогов из Citus (SQL, distributed Postgres) в Tarantool (In-Memory СУБД) с переносом бизнес-логики в UDF (Lua хранимые процедуры).

---

## Архитектура

### ДО (Citus / SQL)

```
Client (Go service)
        |
        | SQL
        v
Citus Coordinator
        |
        +-------------------+
        |                   |
        v                   v
Shard #1 (Postgres)   Shard #2 (Postgres)
```


### ПОСЛЕ (Tarantool / In-Memory + UDF)

```
Client (Go service)
        |
        | UDF calls (Lua)
        v
Tarantool instance
        |
        +-------------------+
        |                   |
        v                   v
dialog_send()        dialog_list()
        |
        v
In-memory spaces + WAL
```
---

## Структура изменений

### Новые файлы

- tarantool/init.lua — инициализация box, spaces
- tarantool/dialog.lua — UDF (dialog_send, dialog_list)
- loadtest/loadtest.js — нагрузочный тест

---

### Изменённые файлы

- internal/store/dialog_store.go — переход на UDF
- internal/config/config.go — добавлен Tarantool
- main.go — заменён Citus → Tarantool
- docker-compose.yml — удалён Citus, добавлен Tarantool
- go.mod — добавлен tarantool driver

---

## Запуск

### Подготовка

```bash
go mod tidy
docker-compose build app
````

### Запуск инфраструктуры

```bash
docker-compose up -d
docker-compose ps
docker logs -f hl_tarantool
```

---

## Нагрузочное тестирование

```bash
cd loadtest
k6 run --env USERS=100 loadtest.js
```
---

## Параметры нагрузочного тестирования

| Параметр  | Значение              |
| --------- | --------------------- |
| Users     | 100                   |
| Send rate | 800 rps               |
| List rate | 1200 rps              |
| Duration  | 3 min                 |
| Load type | constant-arrival-rate |

---

## Результаты

### Citus (PostgreSQL distributed)

```
http_req_duration:
  avg = 179ms
  p95 = 1.0s
  p99 = 1.82s

http_req_failed:
  2.0%

observations:
- высокая tail latency
- деградация под нагрузкой
- shard coordination overhead
```

---

### Tarantool (In-Memory)

```
http_req_duration:
  avg = 17ms
  p95 = 44ms
  p99 = 391ms

http_req_failed:
  ~2%

observations:
- низкая latency
- стабильный throughput
- WAL становится bottleneck под нагрузкой
```

---

## Наблюдения по Tarantool

```
too long WAL write ~0.6s
readahead limit is reached
```


* WAL становится узким местом
* iproto буфер перегружается
* система входит в saturation режим


## Сравнение

| Метрика | Citus | Tarantool |
|----------|------|------------|
| p95 | ~1s | ~44ms |
| p99 | ~1.8s | ~391ms |
| avg | ~179ms | ~17ms |
| ошибки | ~2% | ~2% |
| tail latency | высокая | умеренная |



## Вывод

- Tarantool значительно быстрее по latency
- Citus хуже в tail latency под нагрузкой
- Tarantool ограничен WAL при write-heavy нагрузке

---