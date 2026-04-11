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

## Тест 1
![!img](load_tests/t1_flotLatenciesOverTime.png)
![!img](load_tests/t1_flotTransactionsPerSecond.png)
![!img](load_tests/t1_flotTotalTPS.png)

## Тест 2
![!img](load_tests/t2_flotLatenciesOverTime.png)
![!img](load_tests/t2_flotTransactionsPerSecond.png)
![!img](load_tests/t2_flotTotalTPS.png)


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

## Тест 1 - после репликации
![!img](load_tests/t1_after_rep_flotLatenciesOverTime.png)
![!img](load_tests/t1_after_rep_flotTransactionsPerSecond.png)
![!img](load_tests/t1_after_rep_flotTotalTPS.png)

## Тест 2 - после репликации
![!img](load_tests/t2_after_rep_flotLatenciesOverTime.png)
![!img](load_tests/t2_after_rep_flotTransactionsPerSecond.png)
![!img](load_tests/t2_after_rep_flotTotalTPS.png)


## Кворумная синхронная репликация

```
...
2026-04-11 13:30:24 2026-04-11 10:30:24.944 GMT [1] LOG:  database system is ready to accept connections
2026-04-11 13:31:05 2026-04-11 10:31:05.083 GMT [103] LOG:  standby "slave1_slot" is now a candidate for quorum synchronous standby
2026-04-11 13:31:05 2026-04-11 10:31:05.083 GMT [103] STATEMENT:  START_REPLICATION SLOT "slave1_slot" 0/5F000000 TIMELINE 1
2026-04-11 13:31:05 2026-04-11 10:31:05.085 GMT [104] LOG:  standby "slave2_slot" is now a candidate for quorum synchronous standby
2026-04-11 13:31:05 2026-04-11 10:31:05.085 GMT [104] STATEMENT:  START_REPLICATION SLOT "slave2_slot" 0/5F000000 TIMELINE 1
...
```