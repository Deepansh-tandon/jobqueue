# jobqueue frontend

## Run UI

```bash
cd frontend
npm run dev
```

Open the URL printed in the terminal (often `http://localhost:3000`).

## Run API (required for login / jobs)

From `jobqueue/`:

```bash
docker compose up -d postgres redis
go run ./cmd/server
```

The console shows a green **API online** badge when `127.0.0.1:8080` responds.

## Flow

1. **Console → Auth** — Register (first time) or Login.
2. **Projects** — Create a project, click **Use**.
3. **Submit** — Pick a job type, submit.
4. **Jobs** / **Failures** — Watch status and DLQ.
