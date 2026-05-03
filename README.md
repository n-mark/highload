# ДЗ 5. Масштабируемая подсистема диалогов

## Архитектура диалогов (Citus)

### Выбор ключа шардирования

Таблица `messages` шардируется по полю `conversation_id`.

**`conversation_id`** — строковый ключ вида `<min_uuid>:<max_uuid>`, где UUID двух участников диалога расставляются в лексикографическом порядке для обеспечения консистентности хеша:

```go
// internal/store/dialog_store.go
func conversationID(a, b uuid.UUID) string {
    as, bs := a.String(), b.String()
    if as < bs {
        return fmt.Sprintf("%s:%s", as, bs)
    }
    return fmt.Sprintf("%s:%s", bs, as)
}
```

Почему именно такой ключ:

| Критерий | Объяснение |
|---|---|
| **Локальность данных** | Все сообщения одного диалога (`A↔B`) всегда попадают на **один шард** |
| **Равномерность** | UUID v4/v7 равномерно распределены, поэтому хэши conversation_id распределяются без «горячих» шардов |
| **«Эффект Леди Гаги»** | Даже если один пользователь ведёт миллион диалогов, каждый диалог имеет **свой** `conversation_id` → его сообщения размазываются по **всем шардам**, а не сваливаются в один |

### Схема таблицы

```sql
CREATE TABLE messages (
    id              uuid        NOT NULL DEFAULT uuidv7(),
    conversation_id text        NOT NULL,  -- ключ шардирования
    from_user_id    uuid        NOT NULL,
    to_user_id      uuid        NOT NULL,
    text            text        NOT NULL,
    created_at      timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (conversation_id, id)      -- distribution col обязан быть в PK
);

SELECT create_distributed_table('messages', 'conversation_id', shard_count => 32);
```

### Решардинг без даунтайма

Citus поддерживает онлайн-ребалансировку шардов через встроенный механизм `citus_rebalance_start()`.

**Процесс:**

1. **Добавить новый воркер** (например, `citus-worker4`) в `docker-compose.yml` по аналогии с существующими:
   ```bash
   docker compose up -d citus-worker4
   ```

2. **Зарегистрировать воркер на координаторе** (приложение продолжает работать):
   ```sql
   SELECT * FROM citus_add_node('citus-worker4', 5432);
   ```

3. **Запустить фоновую ребалансировку:**
   ```sql
   SELECT citus_rebalance_start();
   ```
   Citus копирует шарды на новый узел, затем атомарно переключает метаданные. Старые шарды продолжают обслуживать чтение и запись до момента переключения.

4. **Следить за прогрессом:**
   ```sql
   SELECT * FROM citus_rebalance_status();
   ```

5. **Очистить старые шарды** (после успешного завершения):
   ```sql
   SELECT citus_cleanup_orphaned_shards();
   ```

**Даунтайм = 0** — переключение шарда на новый воркер занимает миллисекунды (только обновление записи в `pg_dist_shard_placement`), а само копирование данных выполняется в фоне.

### API диалогов

| Метод | URL | Описание |
|---|---|---|
| `POST` | `/dialog/{user_id}/send` | Отправить сообщение пользователю `user_id` |
| `GET` | `/dialog/{user_id}/list` | Получить историю диалога с пользователем `user_id` |

Оба эндпойнта требуют JWT-токена (`Authorization: Bearer <token>`).

**Пример запроса:**
```bash
curl -X POST http://localhost:8080/dialog/<target_user_id>/send \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{"text": "Hello!"}'

curl http://localhost:8080/dialog/<target_user_id>/list \
  -H "Authorization: Bearer <token>"
```

### Топология Citus в docker-compose

```
citus-coordinator :5435  ← точка входа приложения
    ├── citus-worker1 :5436
    └── citus-worker2 :5437
    └── citus-worker3 :5438
```

Кластер инициализируется сервисом `citus-setup`, который:
1. Вызывает `citus_set_coordinator_host`
2. Регистрирует воркеров через `citus_add_node`
3. Завершается (`restart: no`) — приложение стартует только после него

---

## Инструкция:
1. Собрать проект с помощью команды `docker compose up -d --build` в корневой директории проекта
2. Импортировать коллекцию `load_tests/hl_homework4.postman_collection.json`в Postman
3. Открыть запрос `1. Register N Users`, в разделе `Scripts` выбрать вкладку `Pre-request`, выставить желаемое кол-во пользователей для создания (`const N`)
4. Запустить тестовый набор с помощью нажатия троеточия напротив заголовка набора, далее выбрать пункт `Run`, затем нажать кнопку `Run homework4`
5. Дождаться окончания. Открыть вкладку `Console Log`. Ознакомиться со статистикой выполнения теста.