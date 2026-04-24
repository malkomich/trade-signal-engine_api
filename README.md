# Trade Signal Engine API

HTTP API for session decisions, windows, live signal events, and operational summaries.

## Stack

- Go 1.24+
- In-memory store for local development
- Realtime Database store for production

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
runtime settings and expects Firebase credentials to be mounted explicitly.

The compose file uses the project name `trade-signal-engine-server`, which keeps the API
container grouped with the edge worker in Dozzle on the Raspberry Pi.

The container now exposes a landing page on `http://localhost:18080/` and the proxy can route
`https://tradesignalengine.backend.synapsesea.com/api` to that page while keeping the REST
routes under `/api/*`.

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
- `FIREBASE_PROJECT_ID`: Firebase project ID for the realtime backend
- `FIREBASE_DATABASE_URL`: Realtime Database URL for production
- `STORE_BACKEND`: `memory`, `rtdb`, or `firestore`
- `NOTIFICATION_BACKEND`: `noop`, `collapse`, or `fcm`
- `FCM_TOPIC`: topic name used when `NOTIFICATION_BACKEND=fcm`, default `trade-signal-engine`

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
- `POST /v1/sessions/{id}/exit`
- `POST /v1/sessions/{id}/reject`
- `POST /v1/sessions/{id}/ack`

## Deployment

The Raspberry Pi deployment workflow runs on merges to `main` and executes on the repository's
Raspberry Pi self-hosted runner.

Configure these GitHub repository secrets before enabling deployment:

- `FIREBASE_SERVICE_ACCOUNT_TRADE_SIGNAL_ENGINE`
- `GCP_CREDENTIALS_JSON`

These repository variables are also useful for tooling and local automation:

- `GOOGLE_CLOUD_PROJECT=trade-signal-engine`
- `GCP_PROJECT_ID=trade-signal-engine`

The API also writes live signal rows to the realtime backend in `signal_events`, which is what the
Firebase-hosted admin dashboard reads for real-time triage.
The public proxy points `https://tradesignalengine.backend.synapsesea.com/api` to this API
container through the local port published by Compose.

## Analytics export

`GET /v1/sessions/{id}/analytics/export` returns a versioned daily export payload with:

- symbol summaries grouped by day
- market summaries rolled up from the symbol summaries
- a stable `daily.analytics.v1` export version and export path
