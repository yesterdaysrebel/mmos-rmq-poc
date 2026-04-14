# mmos-rmq-poc

Go RabbitMQ proof-of-concept for consuming and publishing queue messages in Kubernetes, with HTTP endpoints exposed through Traefik.

## Architecture

Components deployed in nonprod:

- App: `mmos-rmq-poc` deployment (Go service, HTTP on port 8080)
- Broker: `rabbitmq` deployment (RabbitMQ 3.13.6-management, AMQP on port 5672)
- Ingress: Traefik (`public-traefik` ingress class) for public HTTPS access to app HTTP routes

High-level flow:

1. Client sends HTTPS request to Traefik host `mmos-rmq-poc.apps.infra.glueops.onglueops.com`.
2. Traefik routes HTTP traffic to `mmos-rmq-poc` service on port 8080.
3. App handles HTTP endpoints (`/`, `/healthz`, `/status`, `/rmq-status`, `/publish`).
4. App connects to RabbitMQ using AMQP at `rabbitmq-nonprod.infra.svc.cluster.local:5672`.
5. For `/publish`, app publishes a message to queue `mmos.events`.
6. Consumer loop in the same app receives messages from `mmos.events` and ACKs them.

## Protocols and ports

- Internet/client to Traefik: HTTPS (TLS) on 443
- Traefik to app service: HTTP on 8080
- App to RabbitMQ: AMQP 0-9-1 on 5672
- RabbitMQ management plugin: HTTP on 15672 inside RabbitMQ container (not exposed via ingress in current setup)

## RabbitMQ configuration used

Current nonprod values from the deployment config:

- Image: `docker.io/rabbitmq:3.13.6-management`
- Service port: `5672`
- Username: `mmos`
- Password: `mmos-nonprod-pass`
- VHost: `/`
- Ingress: disabled

The app is configured to connect with:

- `RABBITMQ_URL=amqp://mmos:mmos-nonprod-pass@rabbitmq-nonprod.infra.svc.cluster.local:5672/`
- `RABBITMQ_QUEUE=mmos.events`

Queue behavior in code:

- Queue is declared as durable
- Consumer uses explicit ACK
- Consumer automatically reconnects if channel/connection drops

## App endpoints

- `GET /`: service metadata/status
- `GET /healthz`: liveness endpoint (`ok`)
- `GET /status`: queue + consumer info
- `GET /rmq-status`: RabbitMQ connectivity + queue depth + consumer count
- `POST /publish`: publish message body to a queue

Example publish request:

```bash
curl -X POST "https://mmos-rmq-poc.apps.infra.glueops.onglueops.com/publish" \
	-H "Content-Type: application/json" \
	-d '{"queue":"mmos.events","body":"hello world"}'
```

## Deployment layout in this workspace

- App source code and Dockerfile: [mmos-rmq-poc](mmos-rmq-poc)
- App nonprod values: [deployment-configurations/apps/mmos-rmq-poc/envs/nonprod/values.yaml](deployment-configurations/apps/mmos-rmq-poc/envs/nonprod/values.yaml)
- RabbitMQ nonprod values: [deployment-configurations/apps/rabbitmq/envs/nonprod/values.yaml](deployment-configurations/apps/rabbitmq/envs/nonprod/values.yaml)

Notes on deployment behavior:

- The app and RabbitMQ are separate Kubernetes apps (separate values files, separate deployments).
- Traffic from outside cluster only reaches the app through Traefik ingress.
- RabbitMQ stays internal to the cluster (no ingress), reached by Kubernetes service DNS.

## Steps we followed

This is the exact rollout sequence we used for this PoC.

1. Created the Go service with RabbitMQ consumer loop.
2. Added health/status endpoints (`/healthz`, `/`, `/status`) to make runtime state visible.
3. Containerized the app with a multi-stage Docker build and configured GHCR publishing in GitHub Actions.
4. Added deployment values for `mmos-rmq-poc` in nonprod with ingress enabled through Traefik.
5. Deployed app and observed connection failures caused by missing RabbitMQ in cluster and fallback to localhost defaults.
6. Created a separate `rabbitmq` app in deployment-configurations for nonprod.
7. Configured RabbitMQ with explicit credentials (`mmos` / `mmos-nonprod-pass`) and AMQP port `5672`.
8. Updated app `RABBITMQ_URL` to point to in-cluster DNS:
	 `amqp://mmos:mmos-nonprod-pass@rabbitmq-nonprod.infra.svc.cluster.local:5672/`.
9. Verified app connectivity with logs and `/rmq-status` endpoint.
10. Added `POST /publish` endpoint to inject test messages over HTTP and publish to RabbitMQ.
11. Hardened HTTP route behavior so `/` is exact-only and `/publish/` redirects to `/publish`.
12. Validated end-to-end flow: publish request -> queue -> consumer receive + ACK.

Useful validation commands we used:

```bash
curl -sS "https://mmos-rmq-poc.apps.infra.glueops.onglueops.com/rmq-status"

curl -X POST "https://mmos-rmq-poc.apps.infra.glueops.onglueops.com/publish" \
	-H "Content-Type: application/json" \
	-d '{"queue":"mmos.events","body":"hello world"}'

kubectl -n infra logs -f deployment/mmos-rmq-poc-nonprod
```

Expected signals during verification:

- `connected to RabbitMQ and consuming queue=mmos.events`
- `received routing_key=... bytes=... payload=...`
- `/rmq-status` returns `connected: true`

## Local development

Requirements:

- Go 1.23+
- Access to a RabbitMQ broker

Run:

```bash
go mod tidy
go run .
```

Environment variables:

- `RABBITMQ_URL` (default: `amqp://guest:guest@localhost:5672/`)
- `RABBITMQ_QUEUE` (default: `mmos.events`)
- `HTTP_ADDR` (default: `:8080`)
- `CONSUMER_NAME` (default: `mmos-rmq-poc`)

Build image locally:

```bash
docker build -t ghcr.io/glueops/mmos-rmq-poc:dev .
```

## Python AMQP publisher

Use the included Python publisher to send a message over AMQPS (Traefik TCP endpoint):

```bash
pip install -r requirements-publisher.txt
python publish_amqp.py --message "hello from python"
```

Default target is:

- Host: `rabbitmq-amqp.apps.infra.glueops.onglueops.com`
- Port: `443`
- Queue: `mmos.events`

You can override with flags, for example:

```bash
python publish_amqp.py \
	--host rabbitmq-amqp.apps.infra.glueops.onglueops.com \
	--port 443 \
	--username mmos \
	--password mmos-nonprod-pass \
	--vhost / \
	--queue mmos.events \
	--message "hello over amqp"
```

## CI/CD

Workflow: [mmos-rmq-poc/.github/workflows/container-ghcr.yml](mmos-rmq-poc/.github/workflows/container-ghcr.yml)

- Builds on pull requests
- Builds and pushes images on `main`
- Builds and pushes images on tags matching `v*`