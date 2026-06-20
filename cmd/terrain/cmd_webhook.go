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
	"github.com/pmclSF/terrain/internal/terrainconfig"
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

	root := resolveRepoRoot()
	handler := slash.NewHandler(secret, newRealDispatcher(root))
	handler.DismissPolicy = loadDismissPolicy(root)
	announceDismissPolicy(handler.DismissPolicy)

	mux := http.NewServeMux()
	mux.Handle("/webhook", handler)
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

// loadDismissPolicy reads terrain.yaml's slash.dismiss section into the
// slash package's DismissPolicy shape. Missing terrain.yaml and missing
// sections both produce the zero-value (deny-all) policy.
func loadDismissPolicy(root string) slash.DismissPolicy {
	cfg, err := terrainconfig.LoadForRoot(root)
	if err != nil || cfg == nil || cfg.Slash == nil || cfg.Slash.Dismiss == nil {
		return slash.DismissPolicy{}
	}
	return slash.DismissPolicy{
		AllowAuthors:                 cfg.Slash.Dismiss.AllowAuthors,
		AllowAnyoneWithCommentAccess: cfg.Slash.Dismiss.AllowAnyoneWithCommentAccess,
	}
}

// announceDismissPolicy prints a startup banner so adopters see at
// boot whether /dismiss is gated or open. Helps catch silent
// misconfigurations where the receiver runs in production with the
// deny-all default and dismissals appear to "do nothing."
func announceDismissPolicy(p slash.DismissPolicy) {
	switch {
	case p.AllowAnyoneWithCommentAccess:
		fmt.Fprintln(os.Stderr, "  slash policy: /dismiss accepted from any PR commenter (slash.dismiss.allow_anyone_with_comment_access=true).")
	case len(p.AllowAuthors) > 0:
		fmt.Fprintf(os.Stderr, "  slash policy: /dismiss accepted from %d allowlisted authors.\n", len(p.AllowAuthors))
	default:
		fmt.Fprintln(os.Stderr, "  slash policy: /dismiss DENY-ALL (no terrain.yaml slash.dismiss configured). The receiver will reply to every /dismiss with a not-authorized notice.")
		fmt.Fprintln(os.Stderr, "    Set slash.dismiss.allow_authors or slash.dismiss.allow_anyone_with_comment_access in terrain.yaml to enable.")
	}
}
