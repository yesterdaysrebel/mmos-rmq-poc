#!/usr/bin/env python3
"""Publish a single AMQP message to RabbitMQ over TLS (AMQPS).

Defaults are set for the GlueOps nonprod RabbitMQ Traefik TCP endpoint.
Override via CLI flags or environment variables.
"""

import argparse
import os
import ssl
import sys

import pika


def env(name: str, default: str) -> str:
    value = os.getenv(name)
    if value is None or value == "":
        return default
    return value


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Publish one AMQP message")
    parser.add_argument(
        "--host",
        default=env("RABBITMQ_HOST", "rabbitmq-amqp.apps.infra.glueops.onglueops.com"),
        help="RabbitMQ host (default: %(default)s)",
    )
    parser.add_argument(
        "--port",
        type=int,
        default=int(env("RABBITMQ_PORT", "443")),
        help="RabbitMQ port (default: %(default)s)",
    )
    parser.add_argument(
        "--username",
        default=env("RABBITMQ_USER", "mmos"),
        help="RabbitMQ username (default: %(default)s)",
    )
    parser.add_argument(
        "--password",
        default=env("RABBITMQ_PASS", "mmos-nonprod-pass"),
        help="RabbitMQ password",
    )
    parser.add_argument(
        "--vhost",
        default=env("RABBITMQ_VHOST", "/"),
        help="RabbitMQ vhost (default: %(default)s)",
    )
    parser.add_argument(
        "--queue",
        default=env("RABBITMQ_QUEUE", "mmos.events"),
        help="Queue name (default: %(default)s)",
    )
    parser.add_argument(
        "--message",
        default=env("RABBITMQ_MESSAGE", "hello over amqp"),
        help="Message body to publish (default: %(default)s)",
    )
    parser.add_argument(
        "--insecure",
        action="store_true",
        help="Disable TLS certificate validation (not recommended)",
    )
    return parser.parse_args()


def main() -> int:
    args = parse_args()

    credentials = pika.PlainCredentials(args.username, args.password)
    ssl_ctx = ssl.create_default_context()
    if args.insecure:
        ssl_ctx.check_hostname = False
        ssl_ctx.verify_mode = ssl.CERT_NONE

    params = pika.ConnectionParameters(
        host=args.host,
        port=args.port,
        virtual_host=args.vhost,
        credentials=credentials,
        ssl_options=pika.SSLOptions(ssl_ctx, args.host),
        heartbeat=30,
        blocked_connection_timeout=10,
    )

    try:
        connection = pika.BlockingConnection(params)
        channel = connection.channel()
        channel.queue_declare(queue=args.queue, durable=True)
        channel.basic_publish(exchange="", routing_key=args.queue, body=args.message.encode("utf-8"))
        connection.close()
    except Exception as exc:
        print(f"publish failed: {exc}", file=sys.stderr)
        return 1

    print(
        f"published queue={args.queue} bytes={len(args.message.encode('utf-8'))} "
        f"host={args.host}:{args.port} vhost={args.vhost}"
    )
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
