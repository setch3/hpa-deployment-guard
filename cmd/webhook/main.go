package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"k8s-deployment-hpa-validator/internal/webhook"
)

func main() {
	var (
		port     = flag.Int("port", 8443, "Webhook server port")
		certFile = flag.String("cert-file", "/etc/certs/tls.crt", "TLS certificate file")
		keyFile  = flag.String("key-file", "/etc/certs/tls.key", "TLS private key file")
	)
	flag.Parse()

	log.Printf("Starting HPA-Deployment validator webhook server on port %d", *port)

	server, err := webhook.NewServer(*port, *certFile, *keyFile)
	if err != nil {
		log.Fatalf("Failed to create webhook server: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle shutdown gracefully
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigCh
		log.Println("Shutting down webhook server...")
		cancel()
	}()

	if err := server.Start(ctx); err != nil {
		log.Fatalf("Failed to start webhook server: %v", err)
	}
}