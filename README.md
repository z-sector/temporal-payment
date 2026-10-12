# temporal-payment

Пример payment workflow на Go SDK Temporal. Шаги не двигают деньги: они пишут запрос в лог.

## Что запускается в Docker Compose

Долгоживущие сервисы:

- PostgreSQL — persistence Temporal
- Elasticsearch — visibility
- Temporal Server — gRPC `localhost:7233`
- Temporal Web UI — http://localhost:8080

`temporal-admin-tools` и `temporal-create-namespace` — одноразовые init-контейнеры. Они создают схему Postgres, индекс Elasticsearch и namespace `default`, затем завершаются. Worker и client в compose нет.

Скрипты схемы взяты из [temporalio/samples-server](https://github.com/temporalio/samples-server/tree/main/compose). В `create-namespace.sh` исправлена опечатка `MAX_ATTdMPTS`, из-за которой повтор создания namespace падал при `set -u`.

## Запуск

```bash
docker compose up
```

Дождитесь в логе `Namespace 'default' created` или `Namespace 'default' already exists`.

В другом терминале запустите worker:

```bash
go run ./worker
```

Клиент отправляет `from`, `to`, `value` и ключ идемпотентности:

```bash
go run ./starter -from bob -to carol -value 100 -idempotency-key pay-ok-1
```

Адрес кластера по умолчанию — `127.0.0.1:7233`. Другой адрес задаётся через `TEMPORAL_ADDRESS`.

Ключ идемпотентности становится Workflow ID. Повторный старт с тем же `-idempotency-key` не создаёт второй workflow. Для каждого шага workflow передаёт свой стабильный ключ, один и тот же на всех retry:

- `{idempotency-key}:hold`
- `{idempotency-key}:antifraud`
- `{idempotency-key}:transfer`
- `{idempotency-key}:refund`

## Сценарии

Префиксы проверяются как есть, строчные `f` и `a`.

- `bob` → `carol` — холд, антифрод, перевод. Статус `completed`.
- `frank` → `carol` — `from` начинается с `f`, холд отвечает «недостаточно средств». Возврата нет. Статус `insufficient_funds`.
- `alice` → `carol` или `bob` → `ann` — антифрод не проходит, workflow возвращает холд. Статус `antifraud_rejected`.

На каждой попытке activity результат срабатывает с вероятностью 30%. Иначе activity пишет в лог `Unavailable` вместе с ключом идемпотентности шага и возвращает retryable-ошибку. Temporal повторяет вызов: первая пауза 1 секунда, дальше backoff до 5 секунд, без лимита попыток.

В логе worker видны и `Unavailable`, и итоговый результат шага. История workflow открывается в Web UI.
