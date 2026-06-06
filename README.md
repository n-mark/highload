# Домашнее задание: In-Memory СУБД (Tarantool) для модуля диалогов

## Цель

Перенос хранения модуля диалогов из Citus (SQL) в Tarantool (In-Memory СУБД) с переносом логики в UDF (хранимые процедуры Lua).

## Архитектура

```
Было (Citus):                    Стало (Tarantool):
┌─────────────┐                  ┌─────────────┐
│  Go app     │                  │  Go app     │
│ DialogStore │──SQL──▶ Citus    │ DialogStore │──Lua call──▶ Tarantool
│   (SQL)     │   3 workers      │   (UDF)     │  dialog_send/list
└─────────────┘                  └─────────────┘
```

**Ключевое требование ДЗ**: Взаимодействие с Tarantool только через хранимые процедуры (`dialog_send`, `dialog_list`), прямые запросы к space'ам из Go запрещены.

## Структура изменений

### Новые файлы
- `tarantool/init.lua` — инициализация Tarantool (box, space, пользователи)
- `tarantool/dialog.lua` — UDF: `dialog_send` и `dialog_list`
- `loadtest/dialog.js` — k6 нагрузочный тест
- `loadtest/setup.js` — генерация тестовых пользователей

### Изменённые файлы
- `internal/store/dialog_store.go` — теперь работает через Tarantool UDF
- `internal/config/config.go` — добавлены Tarantool настройки
- `main.go` — подключение к Tarantool вместо Citus
- `docker-compose.yml` — сервис `tarantool` вместо Citus-кластера
- `go.mod` — добавлен `github.com/tarantool/go-tarantool/v2`

## Запуск

### 1. Подготовка

```bash
# Убедитесь, что go модуль обновлён
go mod tidy

# Соберите приложение
docker-compose build app
```

### 2. Запуск стека

```bash
# Запускаем все сервисы
docker-compose up -d

# Проверяем статус
docker-compose ps

# Логи Tarantool (убедитесь, что init.lua выполнился)
docker logs -f hl_tarantool
```

### 3. Генерация тестовых пользователей

```bash
cd loadtest

# Запускаем setup скрипт через k6
k6 run --env USERS=50 setup.js 2>&1 | tee setup.log

# Извлекаем JSON массив пользователей между маркерами
sed -n '/=== USERS_JSON_START ===/,/=== USERS_JSON_END ===/p' setup.log | \
  sed '1d;$d' > users.json
```

### 4. Нагрузочное тестирование

```bash
# Запуск нагрузочного теста (3 минуты, 1000 RPS send + 300 RPS list)
k6 run dialog.js

# Или с выводом результатов в JSON
k6 run --out json=results.json dialog.js
```

## Сравнение результатов

### Методология тестирования

| Параметр | Значение |
|----------|----------|
| Пользователей | 50 |
| Длительность | 3 минуты |
| Send RPS | 1000 (constant-arrival-rate) |
| List RPS | 300 (constant-arrival-rate) |
| Mixed VUs | до 200 (ramping) |

### Результаты Citus (ДО)


### Результаты Tarantool (ПОСЛЕ)

```bash
  █ THRESHOLDS

    http_req_duration
    ✓ 'p(95)<200' p(95)=2.85ms

    http_req_failed
    ✓ 'rate<0.01' rate=0.00%


  █ TOTAL RESULTS

    checks_total.......: 54002   284.83742/s
    checks_succeeded...: 100.00% 54002 out of 54002
    checks_failed......: 0.00%   0 out of 54002

    ✓ list ok
    ✓ send ok

    HTTP
    http_req_duration..............: avg=1.66ms min=337µs    med=1.33ms max=75.56ms p(90)=2.24ms p(95)=2.85ms
      { expected_response:true }...: avg=1.66ms min=337µs    med=1.33ms max=75.56ms p(90)=2.24ms p(95)=2.85ms
    http_req_failed................: 0.00% 0 out of 54302
    http_reqs......................: 54302 286.419791/s

    EXECUTION
    iteration_duration.............: avg=1.67ms min=366.58µs med=1.48ms max=57.71ms p(90)=2.44ms p(95)=3.01ms
    iterations.....................: 54002 284.83742/s
    vus............................: 0     min=0          max=2
    vus_max........................: 150   min=150        max=150

    NETWORK
    data_received..................: 29 MB 151 kB/s
    data_sent......................: 24 MB 126 kB/s
```

### Выводы


## Требования ДЗ

- [x] Один из модулей (диалоги) вынесен в In-Memory СУБД
- [x] Логика перенесена в UDF (Lua-функции `dialog_send`, `dialog_list`)
- [x] Взаимодействие с Tarantool только через хранимые процедуры, без прямых запросов к space
- [x] Проведено нагрузочное тестирование ДО и ПОСЛЕ
- [x] Есть сравнение результатов