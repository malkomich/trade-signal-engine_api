# Trade Signal Engine API

HTTP API for session decisions, windows, and operational summaries.

## Stack

- Go 1.24+
- In-memory store for local development
- Firestore store for production

## Run

```bash
make run
```

## Test

```bash
make test
```

## Build

```bash
make build
```

## Environment

- `HTTP_ADDR`: bind address, default `:8080`
- `ENVIRONMENT`: runtime environment label, default `local`
- `FIREBASE_PROJECT_ID`: Firebase project ID for Firestore
- `STORE_BACKEND`: `memory` or `firestore`
- `NOTIFICATION_BACKEND`: `noop` or `collapse`

## API

- `GET /healthz`
- `GET /readyz`
- `GET /v1/decisions?session_id=...`
- `POST /v1/decisions`
- `GET /v1/sessions/{id}`
- `PUT /v1/sessions/{id}`
- `GET /v1/sessions/{id}/windows`
- `GET /v1/sessions/{id}/analytics`
- `POST /v1/sessions/{id}/accept`
- `POST /v1/sessions/{id}/reject`
- `POST /v1/sessions/{id}/ack`
