# temporal-payment

A payment workflow example built with the Temporal Go SDK. The steps do not move money; they log the request.

## Docker Compose

Long-running services:

- PostgreSQL — Temporal persistence
- Elasticsearch — visibility
- Temporal Server — gRPC `localhost:7233`
- Temporal Web UI — http://localhost:8080

`temporal-admin-tools` and `temporal-create-namespace` are one-shot init containers. They create the Postgres schema, the Elasticsearch index, and the `default` namespace, then exit. Start the worker and client with Go; they are not part of Compose.

Schema scripts come from [temporalio/samples-server](https://github.com/temporalio/samples-server/tree/main/compose).

## Run

```bash
docker compose up
```

Wait until the log says `Namespace 'default' created` or `Namespace 'default' already exists`.

In another terminal, start the worker:

```bash
go run ./worker
```

The client sends `from`, `to`, `value`, and an idempotency key:

```bash
go run ./starter -from bob -to carol -value 100 -idempotency-key pay-ok-1
```

The cluster address defaults to `127.0.0.1:7233`. Set `TEMPORAL_ADDRESS` to use another address.

The idempotency key becomes the workflow ID. Starting again with the same `-idempotency-key` does not create a second workflow. Each step receives its own stable key, and retries of that step reuse it:

- `{idempotency-key}:hold`
- `{idempotency-key}:antifraud`
- `{idempotency-key}:transfer`
- `{idempotency-key}:refund`

## Scenarios

Prefixes are matched exactly, lowercase `f` and `a`.

- `bob` to `carol` — hold, antifraud, transfer. Status `completed`.
- `frank` to `carol` — `from` starts with `f`, so the hold reports insufficient funds. There is no refund. Status `insufficient_funds`.
- `alice` to `carol`, or `bob` to `ann` — antifraud rejects the payment and the workflow refunds the hold. Status `antifraud_rejected`.

Each activity attempt returns the real result with a 30% probability. Otherwise the activity logs `Unavailable` together with the step idempotency key and returns a retryable error. Temporal retries the call, starting with a 1 second pause and backing off up to 5 seconds, with no attempt limit.

The worker log shows both `Unavailable` and the final step result. The workflow history is in the Web UI.
