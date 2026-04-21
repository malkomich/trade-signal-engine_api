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

## Run in Docker

```bash
docker compose up -d --build
```

Provide a `.env` file or exported environment variables before running the stack:

```bash
FIREBASE_PROJECT_ID=your-firebase-project-id
FIREBASE_CREDENTIALS_FILE=/absolute/path/to/firebase-service-account.json
API_PORT=18080
```

The compose file is production-oriented for the Raspberry Pi, so it defaults to `production`
runtime settings and expects Firestore credentials to be mounted explicitly.

The compose file uses the project name `trade-signal-engine-server`, which keeps the API
container grouped with the edge worker in Dozzle on the Raspberry Pi.

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
- `GET /v1/sessions/{id}/analytics/export`
- `POST /v1/sessions/{id}/accept`
- `POST /v1/sessions/{id}/reject`
- `POST /v1/sessions/{id}/ack`

## Deployment

The Raspberry Pi deployment workflow runs on merges to `main` and expects the repository to be
checked out under `/opt/trade-signal-engine/api` on the target host.

Configure these GitHub repository secrets before enabling deployment:

- `RASPBERRY_PI_HOST`
- `RASPBERRY_PI_USER`
- `RASPBERRY_PI_SSH_KEY`
- `RASPBERRY_PI_HOST_FINGERPRINT`

The public proxy points `https://tradesignalengine.backend.synapsesea.com` to this API container
through the local port published by Compose.

## Analytics export

`GET /v1/sessions/{id}/analytics/export` returns a versioned daily export payload with:

- symbol summaries grouped by day
- market summaries rolled up from the symbol summaries
- a stable `daily.analytics.v1` export version and export path
