# temporal-payment

Local Temporal stack for a Go payment workflow: Cassandra stores workflow history, Elasticsearch stores the visibility index, and the Web UI reads both through the Temporal frontend.

From `compose/`:

```bash
docker compose up
```

| Service | Address |
| --- | --- |
| Temporal gRPC | `localhost:7233` |
| Web UI | http://localhost:8080 |
| Cassandra | `localhost:9042` |
| Elasticsearch | `localhost:9200` |

The Go module uses Go 1.26.9.
