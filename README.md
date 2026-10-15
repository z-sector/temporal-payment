# temporal-payment

A payment workflow example built with the Temporal Go SDK. The steps do not move money; they log the request.

## Docker Compose

Long-running services:

- PostgreSQL — Temporal persistence
- Elasticsearch — visibility
- Temporal Server — gRPC `localhost:7233`
- Temporal Web UI — http://localhost:8080

`temporal-admin-tools` and `temporal-create-namespace` are one-shot init containers. They create the Postgres schema, the Elasticsearch index, and the `default` namespace, then exit. The worker and the starter are not part of Compose. Start them with `make`.

Schema scripts come from [temporalio/samples-server](https://github.com/temporalio/samples-server/tree/main/compose).

## Run

`make help` lists the targets.

Start the cluster in the background:

```bash
make up
```

That runs `docker compose up -d`. The namespace container keeps going after `make up` returns. Wait until its log says `Namespace 'default' created` or `Namespace 'default' already exists`:

```bash
docker compose logs -f temporal-create-namespace
```

In another terminal, start the worker and leave it running:

```bash
make worker
```

The worker holds that terminal. From the repository root, in any other terminal, run one payment or every scenario:

```bash
make scenario-ok
make scenarios
```

`make scenario-ok` is the one payment. `make scenarios` runs `scenario-ok`, `scenario-nofunds`, `scenario-fraud-from`, `scenario-fraud-to`, and `scenario-transfer-down`. The last of those exits non-zero, so `make scenarios` does too.

Stop the cluster and delete its volumes:

```bash
make down
```

That runs `docker compose down -v`.

The cluster address defaults to `127.0.0.1:7233`. Set `TEMPORAL_ADDRESS` to use another address.

The idempotency key becomes the workflow ID. Starting again with the same key does not create a second workflow. Each `make scenario-*` call appends a new suffix, so the same target can be run again. Set `KEY` to pin the workflow ID:

```bash
make scenario-ok KEY=pay-ok-1
```

Each step receives its own stable key, and retries of that step reuse it:

- `{idempotency-key}:hold`
- `{idempotency-key}:antifraud`
- `{idempotency-key}:transfer`
- `{idempotency-key}:refund`

## Scenarios

Prefixes are matched exactly, lowercase `f`, `a`, and `u`.

Hold, antifraud, and transfer. Status `completed`:

```bash
make scenario-ok
```

`from` starts with `f`, so the hold reports insufficient funds. There is no refund. Status `insufficient_funds`:

```bash
make scenario-nofunds
```

`from` or `to` starts with `a`, so antifraud rejects the payment and the workflow refunds the hold. Status `antifraud_rejected`:

```bash
make scenario-fraud-from
make scenario-fraud-to
```

`make scenarios` runs those targets one after another and ends with `scenario-transfer-down`. Each gets its own key.

`from` starts with `u`, so hold and antifraud run, then transfer returns `Unavailable` on each of three attempts and the workflow fails. The starter exits with that error, and so does `make scenarios`. The running worker remembers the failure count for `{idempotency-key}:transfer`. Leave that process running and reset the workflow in the Web UI to the workflow task before Transfer, or to the first workflow task. Reset starts a new run with the same workflow ID, the same worker performs the transfer, and the run completes. Restarting the worker drops the count, and the reset fails again.

```bash
make scenario-transfer-down
```

Each activity attempt returns the real result with a 30% probability. Otherwise the activity logs `Unavailable` together with the step idempotency key and returns a retryable error. Temporal retries the call, starting with a 1 second pause and backing off up to 5 seconds, with no attempt limit. The `u` transfer does not use that probability: every attempt is `Unavailable`, and the workflow stops after three.

The worker log shows both `Unavailable` and the final step result. The workflow history is in the Web UI.
