package service

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/aegis/aegis/internal/health"
)

const defaultListenAddress = ":8080"

func Run(name string) error {
	listenAddress := os.Getenv("AEGIS_LISTEN_ADDR")
	if listenAddress == "" {
		listenAddress = defaultListenAddress
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	listener, err := net.Listen("tcp", listenAddress)
	if err != nil {
		return err
	}

	server := &http.Server{
		Handler:           health.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	serverErrors := make(chan error, 1)
	go func() {
		logger.Info("service listening", "service", name, "address", listener.Addr().String())
		serverErrors <- server.Serve(listener)
	}()

	select {
	case err := <-serverErrors:
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}
	case <-ctx.Done():
		shutdownContext, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownContext); err != nil {
			return err
		}
		if err := <-serverErrors; !errors.Is(err, http.ErrServerClosed) {
			return err
		}
	}

	return nil
}
