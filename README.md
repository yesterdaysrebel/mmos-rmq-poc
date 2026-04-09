# mmos-rmq-poc

Simple Go RabbitMQ consumer proof-of-concept that is ready to containerize and publish to GHCR.

## What it does

- Connects to RabbitMQ using `RABBITMQ_URL`
- Declares a durable queue (`RABBITMQ_QUEUE`)
- Consumes messages and acknowledges them
- Exposes a health endpoint at `/healthz` on `HTTP_ADDR` (default `:8080`)

## Local run

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

## Build image locally

```bash
docker build -t ghcr.io/glueops/mmos-rmq-poc:dev .
```

## GHCR publishing

This repo includes a workflow at `.github/workflows/container-ghcr.yml` that:

- Builds on PRs
- Builds and pushes on `main`
- Builds and pushes on tags matching `v*`

Image naming:

- `ghcr.io/<owner>/<repo>` (lower-cased by workflow)

For organization packages, ensure repository Actions have permission to publish packages.

## Kubernetes example

Manifests are under `k8s/`:

- `k8s/secret-example.yaml` for RabbitMQ URL secret
- `k8s/deployment.yaml` for app deployment

Apply (after updating secret values):

```bash
kubectl apply -f k8s/secret-example.yaml
kubectl apply -f k8s/deployment.yaml
```

## Notes

- Traefik and ingress can be added later for HTTP management UI or TCP AMQP routing.
- This app only needs RabbitMQ reachability from inside the cluster/network.