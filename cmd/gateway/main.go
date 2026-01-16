package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/s3-access-control-adapter/internal/audit"
	"github.com/s3-access-control-adapter/internal/auth"
	"github.com/s3-access-control-adapter/internal/config"
	"github.com/s3-access-control-adapter/internal/policy"
	"github.com/s3-access-control-adapter/internal/proxy"
)

func main() {
	configPath := flag.String("config", "configs/gateway.yaml", "Path to gateway configuration file")
	flag.Parse()

	// Load configuration
	cfg, err := config.LoadGatewayConfig(*configPath)
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	log.Printf("Starting S3 Access Control Adapter Gateway on port %d", cfg.Server.Port)

	// Initialize credential store
	credStore, err := auth.NewInMemoryCredentialStore(cfg.CredentialsFile)
	if err != nil {
		log.Fatalf("Failed to initialize credential store: %v", err)
	}
	log.Printf("Loaded credentials from %s", cfg.CredentialsFile)

	// Initialize signature validator
	sigValidator := auth.NewSignatureValidator()

	// Initialize policy engine
	policyEngine, err := policy.NewEngine(cfg.PoliciesFile)
	if err != nil {
		log.Fatalf("Failed to initialize policy engine: %v", err)
	}
	log.Printf("Loaded policies from %s", cfg.PoliciesFile)

	// Initialize S3 client
	ctx := context.Background()
	s3Client, err := proxy.NewS3Client(ctx, &cfg.AWS)
	if err != nil {
		log.Fatalf("Failed to initialize S3 client: %v", err)
	}
	if cfg.AWS.Endpoint != "" {
		log.Printf("Connected to S3 endpoint: %s", cfg.AWS.Endpoint)
	} else {
		log.Printf("Connected to AWS S3 in region: %s", cfg.AWS.Region)
	}

	// Initialize audit logger
	auditLogger, err := audit.NewLogger(&cfg.Audit)
	if err != nil {
		log.Fatalf("Failed to initialize audit logger: %v", err)
	}
	defer auditLogger.Close()
	if cfg.Audit.Enabled {
		log.Printf("Audit logging enabled, output: %s", cfg.Audit.Output)
	}

	// Create gateway handler
	gateway := proxy.NewGateway(credStore, sigValidator, policyEngine, s3Client, auditLogger)

	// Create HTTP server
	server := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Server.Port),
		Handler:      gateway,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
	}

	// Start server in goroutine
	go func() {
		log.Printf("Server listening on %s", server.Addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")

	// Graceful shutdown
	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.Server.ShutdownTimeout)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("Server shutdown error: %v", err)
	}

	// Wait a bit for pending requests
	time.Sleep(100 * time.Millisecond)

	log.Println("Server stopped")
}
