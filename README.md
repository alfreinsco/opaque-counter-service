# Opaque Counter Service

Tiny public counter collector written in Go using only the standard library.

## Behaviour

- `GET /x/{opaque-token}` increments one counter.
- `POST /x/{opaque-token}` increments the same way.
- Public responses are always `204 No Content`.
- Invalid paths/tokens, unsupported HTTP methods, and Redis errors still look like `204` to the caller.
- Redis error details are written only to server logs.
- `GET /stats` returns all counters as JSON for internal/admin use.
- The service does not know whether a token means a view, download, click, news slug, application view, etc.

A token is simply the counter ID. Keep the semantic mapping in the application that owns the counter.

Example mapping in your main application:

```text
KDfR4t... => app:amboina:view
Ibo7Wc... => news:gempa-ambon:view
Fo0a9N... => document:guide:download
```

Redis only sees:

```text
c:v1:KDfR4t... = 18291
c:v1:Ibo7Wc... = 901
```

## Start with Docker

```bash
docker compose up -d --build
```

The HTTP service is bound to `127.0.0.1:8080` by default so it can sit behind Nginx/Traefik.

## Generate random tokens

If Go is installed locally:

```bash
go run ./cmd/token -n 5
```

Each default token contains 24 random bytes encoded with URL-safe base64 without padding.

## Increment

POST:

```bash
curl -i -X POST http://127.0.0.1:8080/x/YOUR_RANDOM_TOKEN
```

GET:

```bash
curl -i http://127.0.0.1:8080/x/YOUR_RANDOM_TOKEN
```

Both return:

```text
HTTP/1.1 204 No Content
```

## Read counters internally

Read all counters through HTTP:

```bash
curl http://127.0.0.1:8080/stats
```

Example response:

```json
{"counters":[{"token":"YOUR_RANDOM_TOKEN","count":3}],"total":1}
```

The endpoint is only bound to localhost by the default Docker configuration. Add authentication at your reverse proxy before exposing it publicly.

Read one count directly from Redis:

```bash
docker compose exec redis redis-cli GET 'c:v1:YOUR_RANDOM_TOKEN'
```

Or let your admin/backend connect to Redis separately.

## Important limitation

This service intentionally has no authentication. Anyone who learns a valid opaque token can replay it and inflate that counter. Opaque/random URLs hide semantics; they do not prove that a request is legitimate.

Also, GET may be triggered by crawlers, prefetchers, link scanners, or proxies. Prefer POST in your own applications and use GET only when you specifically need it.

## Configuration

| Variable | Default | Purpose |
|---|---|---|
| `LISTEN_ADDRESS` | `:8080` | HTTP listen address |
| `REDIS_ADDRESS` | `redis:6379` | Redis TCP address |
| `REDIS_PASSWORD` | empty | Optional Redis password |
| `REDIS_DB` | `0` | Redis database number |
| `KEY_PREFIX` | `c:v1:` | Prefix for Redis counter keys |
| `PATH_PREFIX` | `/x/` | Public opaque route prefix |
| `STATS_PATH` | `/stats` | Internal endpoint for reading all counters |
| `MIN_TOKEN_LENGTH` | `20` | Minimum accepted token length |
| `MAX_TOKEN_LENGTH` | `96` | Maximum accepted token length |

## Why Redis only?

For a service whose only job is incrementing counters, Redis with AOF persistence keeps the architecture small and fast. If later you need daily analytics/history, add a background worker that periodically rolls Redis totals into PostgreSQL/MySQL rather than making the public request path heavier.
