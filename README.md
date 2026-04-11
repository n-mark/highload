# ДЗ 3. Репликация

>
> 1. *Выбираем 2 запроса на чтение (/user/get/{id} и /user/search из спецификации).*
> *Составить план нагрузочного тестирования, который шлет запросы на эти api.*
> 2. *Создаем нагрузку на чтение с помощью составленного на предыдущем шаге плана, делаем замеры.*

<br>
Поскольку тест осуществлялся в локальном окружении на одной физической машине, для достижения адекватных результатов контейнеры БД были запущены с ограничениями по ресурсам. Запуск без ограничений по ресурсам в режиме репликации приводил к падению пропускной способности из-за конкуренции.

```yaml
    ...
      resources:
        limits:
          cpus: '1'
          memory: 1g
    ...
```

В связи с этим показатели задержки и пропускной способности значительно отличаются от результатов, полученных в ДЗ 2 - ограничение ресурсов привело к падению числа транзакций и увеличению таймингов обработки запроса.

## Тест 1 (100 VU, 10 MIN, 1 SEC RAMP UP, 300 SEC DURATION)
Latency. Верхняя черта - запрос поиска по списку `/profile/list`, нижняя - получение пользователя по id `/profile/{id}`
![!img](load_tests/t1_flotLatenciesOverTime.png)
Кол-во транзакций в секунду. Верхняя черта - запрос поиска по списку `/profile/list`, нижняя - получение пользователя по id `/profile/{id}`
![!img](load_tests/t1_flotTransactionsPerSecond.png)
Общее кол-во транзакций в секунду
![!img](load_tests/t1_flotTotalTPS.png)

## Тест 2 (1000 VU, 10 MIN, 0 SEC RAMP UP, 600 SEC DURATION)
Latency. Верхняя черта - запрос поиска по списку `/profile/list`, нижняя - получение пользователя по id `/profile/{id}`
![!img](load_tests/t2_flotLatenciesOverTime.png)
Кол-во транзакций в секунду. Верхняя черта - запрос поиска по списку `/profile/list`, нижняя - получение пользователя по id `/profile/{id}`
![!img](load_tests/t2_flotTransactionsPerSecond.png)
Общее кол-во транзакций в секунду
![!img](load_tests/t2_flotTotalTPS.png)

<br>

Данный результат сначала показался мне странным - как количество обращений для получения пользователя по ключу может быть меньше, чем кол-во поисков по списку? Однако затем я всмомнил, что в тесте настроен `If Controller`, пропускающий вызов эндпоинта `/profile/{id}`, если поиск случайных значений по `/profile/list` не дал результатов.

## После репликации

> 3. *Настроить 2 слейва и 1 мастер. Включить потоковую репликацию.*

<br>
Добавляем в docker-compose создание контейнеров с мастером и слейвами:

```yml
...
...
postgres-master:
    image: postgres:18.3
    container_name: hl_postgres_master
...
...
postgres-slave1:
    image: postgres:18.3
    container_name: hl_postgres_slave1
...
...
postgres-slave2:
    image: postgres:18.3
    container_name: hl_postgres_slave2
...
...
```

Добавляем конфиг файлы с необходимыми для репликации настройками:
<p>
db/master/pg_hba.conf

```
...
# Allow replication connections from slaves
host    replication     replicator      0.0.0.0/0               md5
...
```

db/master/postgresql.conf
```
...
wal_level = replica
max_wal_senders = 10
...
...
max_replication_slots = 10
...
```

db/slave/postgresql.conf
```
...
wal_level = replica
...
...
```

Создаем пользователя "replicator" и слоты для репликации:
```sql
-- Create replication user for streaming replication
DO
$$
BEGIN
   IF NOT EXISTS (SELECT FROM pg_catalog.pg_roles WHERE rolname = 'replicator') THEN
      CREATE ROLE replicator WITH REPLICATION LOGIN PASSWORD 'replicator_pass';
   END IF;
END
$$;

-- Create replication slot for slave1
SELECT pg_create_physical_replication_slot('slave1_slot');

-- Create replication slot for slave2
SELECT pg_create_physical_replication_slot('slave2_slot');

```
<br>

> 4. *Настроить бекенд приложение на работу с реплицированной бд*


Routing запросов между master/slave реализован **на уровне application layer вручную**, через разделение пулов подключений для записи и чтения.

* в Go отсутствует стандартный framework-аналог `@Transactional(readOnly=true/false)`;
* управление соединениями и маршрутизацией SQL-запросов обычно осуществляется явно в коде;
* это обеспечивает прозрачный контроль над тем, какие запросы направляются на master, а какие — на replica.

В конфигурацию приложения добавлена поддержка списка replica-hosts:

`internal/config/config.go`

```go
type Config struct {
    ...
    DBReplicaHosts string
    ...
}
```

Replica DSN формируются динамически:

```go
func (c Config) ReplicaDSNs() []string {
    if c.DBReplicaHosts == "" {
        return []string{c.DSN()}
    }

    hosts := strings.Split(c.DBReplicaHosts, ",")
    dsns := make([]string, 0, len(hosts))

    for _, h := range hosts {
        h = strings.TrimSpace(h)
        if h != "" {
            dsns = append(dsns, fmt.Sprintf(
                "postgres://%s:%s@%s/%s?sslmode=disable",
                c.DBUser, c.DBPassword, h, c.DBName,
            ))
        }
    }

    if len(dsns) == 0 {
        return []string{c.DSN()}
    }

    return dsns
}
```
<br>

Для балансировки чтения между несколькими replica реализован `ReplicaPool`, работающий по алгоритму **Round Robin**:

`internal/store/replica_pool.go`

```go
type ReplicaPool struct {
    pools []*pgxpool.Pool
    next  atomic.Uint64
}
```

Выбор replica:

```go
func (r *ReplicaPool) pool() *pgxpool.Pool {
    if len(r.pools) == 1 {
        return r.pools[0]
    }

    i := r.next.Add(1) % uint64(len(r.pools))
    return r.pools[i]
}
```
<br>

В store-слое используются отдельные подключения:

```go
type UserStore struct {
    master  DB
    replica DB
}
```

Операции записи направляются на master

```go
func (s *UserStore) Create(...) {
    row := s.master.QueryRow(ctx,
        `INSERT INTO users (...) VALUES (...) RETURNING ...`,
    )
}
```

Операции чтения направляются на replica

```go
func (s *UserStore) GetByID(...) {
    err := s.replica.QueryRow(ctx,
        `SELECT ... FROM users WHERE id=$1`,
        id,
    )
}
```

```go
func (s *UserStore) GetByUsername(...) {
    err := s.replica.QueryRow(ctx,
        `SELECT ... FROM users WHERE username=$1`,
        username,
    )
}
```
<br>

> 5. *Создаем нагрузку на чтение с помощью составленного на предыдущем шаге плана, делаем замеры. Добавить сравнение результатов в отчет.*

## Тест 1 (100 VU, 10 MIN, 1 SEC RAMP UP, 300 SEC DURATION) - после репликации
Latency. Верхняя черта - запрос поиска по списку (/profile/list), нижняя - получение пользователя по id (/profile/{id})
![!img](load_tests/t1_after_rep_flotLatenciesOverTime.png)
Кол-во транзакций в секунду. Верхняя черта - запрос поиска по списку (/profile/list), нижняя - получение пользователя по id (/profile/{id})
![!img](load_tests/t1_after_rep_flotTransactionsPerSecond.png)
Общее кол-во транзакций в секунду
![!img](load_tests/t1_after_rep_flotTotalTPS.png)

## Тест 2 (1000 VU, 10 MIN, 0 SEC RAMP UP, 600 SEC DURATION) - после репликации
Latency. Верхняя черта - запрос поиска по списку (/profile/list), нижняя - получение пользователя по id (/profile/{id})
![!img](load_tests/t2_after_rep_flotLatenciesOverTime.png)
Кол-во транзакций в секунду. Верхняя черта - запрос поиска по списку (/profile/list), нижняя - получение пользователя по id (/profile/{id})
![!img](load_tests/t2_after_rep_flotTransactionsPerSecond.png)
Общее кол-во транзакций в секунду
![!img](load_tests/t2_after_rep_flotTotalTPS.png)

<br>

После настройки репликации виден прирост кол-ва транзакций в секунду примерно на 100-120 единиц.

## Кворумная синхронная репликация

см. ветку `homework3-pt2`