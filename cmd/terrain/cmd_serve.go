package main

import (
	"context"
	"os"
	"os/signal"
	"path/filepath"

	"github.com/pmclSF/terrain/internal/server"
)

func runServe(root string, port int) error {
	absRoot, _ := filepath.Abs(root)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	srv := server.New(absRoot, port)
	return srv.ListenAndServe(ctx)
}
