package main

import (
	"errors"
	stdlog "log"
	"log/slog"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"
	"github.com/oong26/rfarm-be/internal/configs"
	"github.com/oong26/rfarm-be/pkg/config"
	"github.com/oong26/rfarm-be/pkg/logger"
	"github.com/oong26/rfarm-be/pkg/middleware"
)

func main() {
	// Config
	cfg, err := config.Load[configs.Config]()
	if err != nil {
		// use standard library cuz the logger pkg doesn't exists yet
		stdlog.Fatal("failed ", err)
	}

	// Logger
	log := logger.New(logger.Options{
		Level: cfg.Log.Level,
		JSON:  cfg.App.Env != "development",
	})
	slog.SetDefault(log) // makes logger.Info etc. use the configured logger

	if err := run(cfg, log); err != nil {
		log.Error("fatal", "err", err)
		os.Exit(1)
	}
}

func run(cfg *configs.Config, log *slog.Logger) error {
	log.Info("server starting", "addr", cfg.App.Addr, "env", cfg.App.Env)

	r := chi.NewRouter()
	r.Use(middleware.RequestID())
	r.Use(middleware.Recover(log))
	r.Use(middleware.AccessLog(log))
	r.Use(middleware.CORS(cfg.Cors.AllowedOrigins))
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		if _, err := w.Write([]byte("welcome")); err != nil {
			log.Warn("write response failed", "err", err)
		}
	})

	err := http.ListenAndServe(cfg.App.Addr, r)
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}
