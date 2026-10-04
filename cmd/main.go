package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/RitochitGhosh/seatbooking-api/internal/adapters/mongodb"
	"github.com/RitochitGhosh/seatbooking-api/internal/adapters/redis"
	"github.com/RitochitGhosh/seatbooking-api/internal/auth"
	"github.com/RitochitGhosh/seatbooking-api/internal/booking"
	"github.com/RitochitGhosh/seatbooking-api/internal/config"
	"github.com/RitochitGhosh/seatbooking-api/internal/httpapi"
	"github.com/RitochitGhosh/seatbooking-api/internal/movie"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if err := run(logger); err != nil {
		logger.Error("server stopped", "error", err)
		os.Exit(1)
	}
}
func run(logger *slog.Logger) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	rdb, err := redis.NewClient(ctx, cfg.RedisAddress, cfg.RedisPassword)
	if err != nil {
		return err
	}
	defer rdb.Close()
	mongoClient, err := mongodb.NewClient(ctx, cfg.MongoURI)
	if err != nil {
		return err
	}
	defer func() {
		closeCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = mongoClient.Disconnect(closeCtx)
	}()
	db := mongoClient.Database(cfg.MongoDatabase)
	usersStore := auth.NewMongoStore(db)
	moviesStore := movie.NewMongoStore(db)
	bookingsStore := booking.NewMongoStore(db)
	for _, ensure := range []func(context.Context) error{usersStore.EnsureIndexes, moviesStore.EnsureIndexes, bookingsStore.EnsureIndexes} {
		if err := ensure(ctx); err != nil {
			return err
		}
	}
	authService, err := auth.NewService(usersStore, auth.NewRedisStore(rdb))
	if err != nil {
		return err
	}
	if cfg.AdminEmail != "" {
		if err := authService.BootstrapAdmin(ctx, cfg.AdminEmail, cfg.AdminPassword); err != nil {
			return err
		}
	}
	movies := movie.NewService(moviesStore)
	bookings := booking.NewService(bookingsStore, movies)
	server := &http.Server{
		Addr: cfg.Address, Handler: httpapi.New(authService, movies, bookings, rdb, func(ctx context.Context) error { return mongoClient.Ping(ctx, nil) }, cfg.Origin, cfg.SecureCookies, logger),
		ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 15 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 16 << 10,
	}
	shutdown, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	result := make(chan error, 1)
	go func() { result <- server.ListenAndServe() }()
	logger.Info("listening", "address", cfg.Address, "origin", cfg.Origin)
	select {
	case err := <-result:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-shutdown.Done():
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(ctx); err != nil {
			_ = server.Close()
			return err
		}
		return nil
	}
}
