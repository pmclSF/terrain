package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/pmclSF/terrain/internal/slash"
)

// runWebhook starts an HTTP server that receives GitHub webhook
// deliveries, validates signatures, parses slash-command bodies,
// and dispatches them through a Dispatcher. The default dispatcher
// is informational — it returns markdown describing what the verb
// would do without mutating state. Adopters override this in their
// own integration code.
//
// The secret is read from TERRAIN_WEBHOOK_SECRET (no flag — secrets
// in CLI args show up in process lists and shell history). Empty
// secret fails fast.
//
// Graceful shutdown: SIGINT/SIGTERM trigger a 5s drain via
// http.Server.Shutdown. Existing in-flight requests get a chance to
// finish; new connections are refused.
func runWebhook(addr string) error {
	secret := os.Getenv("TERRAIN_WEBHOOK_SECRET")
	if secret == "" {
		return fmt.Errorf("TERRAIN_WEBHOOK_SECRET is required (set the same value GitHub uses in the webhook configuration)")
	}

	mux := http.NewServeMux()
	mux.Handle("/webhook", slash.NewHandler(secret, newRealDispatcher(resolveRepoRoot())))
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("ok"))
	})

	srv := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	// Listen for shutdown signals.
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		fmt.Fprintf(os.Stderr, "terrain webhook server listening on %s (POST /webhook)\n", addr)
		err := srv.ListenAndServe()
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
		close(errCh)
	}()

	select {
	case <-ctx.Done():
		fmt.Fprintln(os.Stderr, "shutdown signal received; draining (5s timeout)...")
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("shutdown: %w", err)
		}
		return nil
	case err, ok := <-errCh:
		if !ok {
			return nil
		}
		return err
	}
}
