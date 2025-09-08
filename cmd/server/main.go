package main

import (
	"context"
	"database/sql"
	"log"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/joho/godotenv"

	"virtualservers/internal/api"
	"virtualservers/internal/repository"
	"virtualservers/internal/service"
)

func main() {
	_ = godotenv.Load() // loads .env if present

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		log.Fatal("DATABASE_URL not set (put it in .env or export it)")
	}
	addr := os.Getenv("HTTP_ADDR")
	if addr == "" {
		addr = ":8080"
	}
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		log.Fatal(err)
	}
	store := &repository.Store{DB: db}
	h := &api.Handler{Store: store}
	//Starting billing daemon
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go service.StartBillingDaemon(ctx, store, 60*time.Second)
	go service.StartIdleReaper(ctx, store, 30*time.Second)
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(60 * time.Second))

	//Routes
	r.Get("/servers", h.ListServers)
	r.Get("/servers/{id}", h.GetServer)
	r.Post("/servers/{id}/action", h.ServerAction)
	r.Get("/servers/{id}/logs", h.GetServerLogs)
	r.Post("/server", h.CreateServer)
	health := &api.HealthHandler{DB: db}
	r.Get("/healthz", health.Healthz)
	r.Get("/readyz", health.Healthz)
	srv := &http.Server{
		Addr:    addr,
		Handler: r,
	}

	go func() {
		log.Printf("HTTP listening on %s", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
		}
	}()
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt)
	<-quit
	ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)
}
