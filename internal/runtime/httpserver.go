package runtime

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"
)

func ServeWithGracefulShutdown(serviceName, addr string, handler http.Handler) error {
	server := &http.Server{
		Addr:    addr,
		Handler: handler,
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- server.ListenAndServe()
	}()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	select {
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
		log.Printf("%s shutdown requested; draining for up to 30s", serviceName)
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			return err
		}
		log.Printf("%s shutdown complete", serviceName)
		return nil
	}
}
