# syntax=docker/dockerfile:1.7

FROM golang:1.23-alpine AS builder
WORKDIR /src

COPY go.mod go.sum* ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags='-s -w' -o /out/mmos-rmq-poc ./main.go

FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /
COPY --from=builder /out/mmos-rmq-poc /mmos-rmq-poc

EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/mmos-rmq-poc"]
