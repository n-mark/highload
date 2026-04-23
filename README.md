# ДЗ 4. Кеширование

## Как работает инвалидация кеша?

Инвалидация кеша в системе реализована по принципу событий (event-driven) с использованием Apache Kafka. Все операции, влияющие на ленту пользователя, публикуют события в топик `feed-events`, а воркер-обработчик обновляет кеш в Redis соответствующим образом.

### Механизмы инвалидации

| Событие | Действие с кешем | Описание |
|---------|------------------|----------|
| `EventPostCreated` | `InsertPost` | Новый пост вставляется в ленты всех подписчиков автора |
| `EventPostUpdated` | `UpdatePost` | Обновляется содержимое поста в Redis (быстрая операция) |
| `EventPostDeleted` | `DeletePost` | Пост удаляется из лент всех пользователей и из кеша |
| `EventFriendRemoved` | `Invalidate` | Полная инвалидация ленты пользователя (friendID) |
| `EventFriendAdded` | `RebuildFeed` | Перестройка ленты пользователя (friendID), чтобы включить посты нового друга |

### Детали реализации

**1. InsertPost** (`cache.go`):
- Пост сохраняется в Redis с ключом `post:<postID>`
- Для каждого подписчика создаётся запись в отсортированном множестве `feeder:<userID>` с score = timestamp поста
- Множество `post_users:<postID>` отслеживает, в чьих лентах содержится пост
- При вставке лишние записи (старше 1000) обрезаются командой `ZRemRangeByRank`

**2. Invalidate** (`cache.go`):
- Выполняет `DEL` ключа `feeder:<userID>`
- Лента будет перестроена при следующем запросе (cache-aside pattern)

**3. DeletePost** (`cache.go`):
- Получает список всех пользователей из `post_users:<postID>`
- Удаляет postID из каждой ленты (`ZRem`)
- Удаляет сам пост и вспомогательные ключи

### Схема ключей Redis

```
feeder:<userID>     -> Sorted Set (postID, timestamp)  - лента пользователя
post:<postID>       -> String (JSON(GetPostDTO))       - детали поста
post_users:<postID> -> Set (userID)                    - кто видит этот пост
users_with_feeds    -> Set (userID)                    - все пользователи с кешированными лентами
```

## Как реализована перестройка кешей из СУБД?

Перестройка кеша выполняется в двух сценариях:

### 1. Отложенная перестройка (Lazy Rebuild)

При cache-miss в `PostService.GetFeed`:
1. Запрос к Redis не находит ленту → возвращается `nil`
2. Выполняется полный запрос к БД: `store.Feed(userID, 1000, 0)`
3. Результат сериализуется и записывается в Redis через `cache.Set()`
4. Следующие запросы обслуживаются из кеша

```go
// internal/services/post_service.go
if posts := s.cache.Get(userID, limit, offset); posts != nil {
    slog.Debug("GETTING FEED FROM CACHE", "USERID", userID)
    return models.FeedResponseDTO{Posts: posts, Source: "cache"}, nil
}
// Cache miss - fetch from DB and populate cache
fullPosts, err := s.store.Feed(ctx, userID, 1000, 0)
// ... serialize and call s.cache.Set(userID, cachePosts)
```

### 2. Принудительная перестройка (Triggered Rebuild)

При добавлении друга (`EventFriendAdded`):
1. Воркер получает событие из Kafka
2. Вызывается `rebuildUserFeed(ctx, userID)`
3. Из СУБД загружаются посты: `postStore.Feed(userID, maxFeedSize, 0)`
4. Лента полностью перезаписывается: `cache.Set(userID, posts)`

```go
// internal/feed/worker.go
func (w *Worker) rebuildUserFeed(ctx context.Context, userID uuid.UUID) {
    posts, err := w.postStore.Feed(ctx, userID, maxFeedSize, 0)
    if err != nil {
        log.Printf("feed worker: failed to rebuild feed for %s: %v", userID, err)
        return
    }
    // Convert to DTOs and write to cache
    w.cache.Set(userID, dtos)
}
```

### 3. Массовая перестройка (RebuildAll)

Для восстановления кеша после сбоя:
```go
func (w *Worker) RebuildAll(ctx context.Context) {
    userIDs := w.cache.AllUserIDs()  // читаем из множества users_with_feeds
    for _, uid := range userIDs {
        w.rebuildUserFeed(ctx, uid)
    }
}
```

### Поток данных при перестройке

```
User Request → Redis (miss) → PostgreSQL → Redis (Set) → Response
                    ↓
              Cache miss logged
                    ↓
         postStore.Feed() получает данные
                    ↓
         models.GetPostDTO{} маппинг
                    ↓
         cache.Set() записывает в Redis
```

## Инструкция:
1. Собрать проект с помощью команды `docker compose up -d --build` в корневой директории проекта
2. Импортировать коллекцию `load_tests/hl_homework4.postman_collection.json`в Postman
3. Открыть запрос `1. Register N Users`, в разделе `Scripts` выбрать вкладку `Pre-request`, выставить желаемое кол-во пользователей для создания (`const N`)
4. Запустить тестовый набор с помощью нажатия троеточия напротив заголовка набора, далее выбрать пункт `Run`, затем нажать кнопку `Run homework4`
5. Дождаться окончания. Открыть вкладку `Console Log`. Ознакомиться со статистикой выполнения теста.