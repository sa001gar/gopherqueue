// Package cli provides the serve command for GopherQueue.
package cli

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/sa001gar/gopherqueue/api"
	"github.com/sa001gar/gopherqueue/persistence"
	"github.com/sa001gar/gopherqueue/recovery"
	"github.com/sa001gar/gopherqueue/scheduler"
	"github.com/sa001gar/gopherqueue/worker"
)

var (
	httpAddr        string
	workers         int
	shutdownTimeout time.Duration
	useBoltDB       bool
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the GopherQueue server",
	Long: `Start the GopherQueue server with all components:
  - HTTP API server
  - Job scheduler
  - Worker pool
  - Recovery manager

Example:
  gq serve --http :8080 --workers 10 --data-dir ./data`,
	RunE: runServe,
}

func init() {
	serveCmd.Flags().StringVar(&httpAddr, "http", ":8080", "HTTP server address")
	serveCmd.Flags().IntVar(&workers, "workers", 10, "number of worker goroutines")
	serveCmd.Flags().DurationVar(&shutdownTimeout, "shutdown-timeout", 30*time.Second, "graceful shutdown timeout")
	serveCmd.Flags().BoolVar(&useBoltDB, "bolt", true, "use BoltDB for persistence (false for in-memory)")
}

func runServe(cmd *cobra.Command, args []string) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	log.Printf("Starting GopherQueue %s", Version)
	log.Printf("  Data directory: %s", dataDir)
	log.Printf("  HTTP address: %s", httpAddr)
	log.Printf("  Workers: %d", workers)

	// Initialize persistence
	var store persistence.JobStore
	var err error

	if useBoltDB {
		store, err = persistence.NewBoltStore(dataDir)
		if err != nil {
			return fmt.Errorf("failed to initialize BoltDB store: %w", err)
		}
		log.Println("  Persistence: BoltDB")
	} else {
		store = persistence.NewMemoryStore()
		log.Println("  Persistence: In-Memory (data will be lost on restart)")
	}
	defer store.Close()

	// Initialize scheduler
	schedConfig := scheduler.DefaultSchedulerConfig()
	sched := scheduler.NewPriorityScheduler(store, schedConfig)
	if err := sched.Start(ctx); err != nil {
		return fmt.Errorf("failed to start scheduler: %w", err)
	}
	log.Println("  Scheduler: Started")

	// Initialize worker pool
	poolConfig := &worker.WorkerConfig{
		Concurrency:         workers,
		HeartbeatInterval:   30 * time.Second,
		JobTimeout:          30 * time.Minute,
		ShutdownGracePeriod: shutdownTimeout,
		DataDir:             dataDir,
	}
	pool := worker.NewSimplePool(store, sched, poolConfig)

	// Register example handlers
	registerExampleHandlers(pool)

	if err := pool.Start(ctx); err != nil {
		return fmt.Errorf("failed to start worker pool: %w", err)
	}
	log.Println("  Worker Pool: Started")

	// Initialize recovery manager
	recoveryConfig := recovery.DefaultRecoveryConfig()
	recoveryMgr := recovery.NewSimpleRecoveryManager(store, recoveryConfig)
	if err := recoveryMgr.Start(ctx); err != nil {
		return fmt.Errorf("failed to start recovery manager: %w", err)
	}
	log.Println("  Recovery Manager: Started")

	// Initialize API server
	apiConfig := &api.ServerConfig{
		Addr:         httpAddr,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
	}
	apiServer := api.NewServer(store, sched, pool, apiConfig)
	if err := apiServer.Start(); err != nil {
		return fmt.Errorf("failed to start API server: %w", err)
	}
	log.Printf("  API Server: Listening on %s", httpAddr)

	log.Println("\nGopherQueue is ready to accept jobs!")
	log.Println("Press Ctrl+C to stop")

	// Wait for shutdown signal
	<-ctx.Done()
	log.Println("\nShutting down...")

	// Graceful shutdown
	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	// Stop in reverse order
	if err := apiServer.Stop(shutdownCtx); err != nil {
		log.Printf("Error stopping API server: %v", err)
	}

	if err := recoveryMgr.Stop(shutdownCtx); err != nil {
		log.Printf("Error stopping recovery manager: %v", err)
	}

	if err := pool.Stop(shutdownCtx); err != nil {
		log.Printf("Error stopping worker pool: %v", err)
	}

	if err := sched.Stop(shutdownCtx); err != nil {
		log.Printf("Error stopping scheduler: %v", err)
	}

	log.Println("GopherQueue stopped")
	return nil
}

// registerExampleHandlers registers example job handlers.
func registerExampleHandlers(pool worker.WorkerPool) {
	// Echo handler - returns the payload
	pool.RegisterHandler("echo", func(ctx context.Context, jctx worker.JobContext) error {
		job := jctx.Job()
		log.Printf("[echo] Processing job %s with payload: %s", job.ID, string(job.Payload))
		jctx.SetOutput(job.Payload)
		return nil
	})

	// Sleep handler - simulates long-running job
	pool.RegisterHandler("sleep", func(ctx context.Context, jctx worker.JobContext) error {
		job := jctx.Job()
		log.Printf("[sleep] Job %s starting", job.ID)

		// Default 5 seconds
		duration := 5 * time.Second
		if len(job.Payload) > 0 {
			if d, err := time.ParseDuration(string(job.Payload)); err == nil {
				duration = d
			}
		}

		// Progress updates
		steps := 10
		stepDuration := duration / time.Duration(steps)
		for i := 1; i <= steps; i++ {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(stepDuration):
				percent := float64(i) / float64(steps) * 100
				jctx.Progress(percent, fmt.Sprintf("Step %d/%d", i, steps))
			}
		}

		log.Printf("[sleep] Job %s completed", job.ID)
		return nil
	})

	// Error handler - always fails (for testing)
	pool.RegisterHandler("error", func(ctx context.Context, jctx worker.JobContext) error {
		return fmt.Errorf("intentional error: %s", string(jctx.Job().Payload))
	})

	log.Println("  Handlers: echo, sleep, error")
}
