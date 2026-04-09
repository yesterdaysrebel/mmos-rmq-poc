package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

type config struct {
	rabbitURL    string
	queueName    string
	httpAddr     string
	consumerName string
}

func loadConfig() config {
	return config{
		rabbitURL:    envOrDefault("RABBITMQ_URL", "amqp://guest:guest@localhost:5672/"),
		queueName:    envOrDefault("RABBITMQ_QUEUE", "mmos.events"),
		httpAddr:     envOrDefault("HTTP_ADDR", ":8080"),
		consumerName: envOrDefault("CONSUMER_NAME", "mmos-rmq-poc"),
	}
}

func main() {
	cfg := loadConfig()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 2)
	go runHealthServer(cfg.httpAddr, errCh)
	go consumeMessages(ctx, cfg, errCh)
	go statusCheckHandler(cfg)

	select {
	case <-ctx.Done():
		log.Println("shutdown signal received")
	case err := <-errCh:
		if err != nil {
			log.Printf("fatal error: %v", err)
			stop()
		}
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	<-shutdownCtx.Done()
	log.Println("service stopped")
}

func statusCheckHandler(cfg config) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{"queue":%q,"consumer":%q,"addr":%q}`,
			cfg.queueName, cfg.consumerName, cfg.httpAddr)
	}
}

func runHealthServer(addr string, errCh chan<- error) {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	srv := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	log.Printf("health server listening on %s", addr)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		errCh <- err
	}
}

func consumeMessages(ctx context.Context, cfg config, errCh chan<- error) {
	for {
		if err := runConsumerOnce(ctx, cfg); err != nil {
			if ctx.Err() != nil {
				return
			}
			log.Printf("consumer disconnected: %v", err)
			select {
			case <-ctx.Done():
				return
			case <-time.After(5 * time.Second):
			}
			continue
		}
		return
	}
}

func runConsumerOnce(ctx context.Context, cfg config) error {
	conn, err := amqp.Dial(cfg.rabbitURL)
	if err != nil {
		return err
	}
	defer conn.Close()

	ch, err := conn.Channel()
	if err != nil {
		return err
	}
	defer ch.Close()

	_, err = ch.QueueDeclare(
		cfg.queueName,
		true,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		return err
	}

	deliveries, err := ch.Consume(
		cfg.queueName,
		cfg.consumerName,
		false,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		return err
	}

	notifyClose := make(chan *amqp.Error, 1)
	ch.NotifyClose(notifyClose)

	log.Printf("connected to RabbitMQ and consuming queue=%s", cfg.queueName)

	for {
		select {
		case <-ctx.Done():
			return nil
		case closeErr := <-notifyClose:
			if closeErr == nil {
				return errors.New("channel closed")
			}
			return closeErr
		case msg, ok := <-deliveries:
			if !ok {
				return errors.New("delivery channel closed")
			}
			log.Printf("received routing_key=%s bytes=%d payload=%q", msg.RoutingKey, len(msg.Body), msg.Body)
			if err := msg.Ack(false); err != nil {
				log.Printf("ack failed: %v", err)
			}
		}
	}
}

func envOrDefault(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}
