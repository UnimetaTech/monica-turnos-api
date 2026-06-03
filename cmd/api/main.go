package main

import (
	"context"
	"log"
	"net/http"

	"monica-turnos-api/internal/config"
	"monica-turnos-api/internal/httpapi"
	"monica-turnos-api/internal/store"
	"monica-turnos-api/internal/syncn8n"

	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	cfg := config.Load()

	if cfg.DatabaseURL == "" {
		log.Fatal("DATABASE_URL no está configurado")
	}

	ctx := context.Background()

	db, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("error conectando a postgres: %v", err)
	}
	defer db.Close()

	if err := db.Ping(ctx); err != nil {
		log.Fatalf("postgres no responde: %v", err)
	}

	repo := store.New(db)
	syncer := syncn8n.New(repo, cfg.N8NSyncWebhookURL, cfg.N8NSyncSecret)
	router := httpapi.NewRouter(repo, syncer, cfg.FrontendOrigin)

	log.Printf("MONICA Turnos API escuchando en http://localhost:%s", cfg.Port)

	if err := http.ListenAndServe(":"+cfg.Port, router); err != nil {
		log.Fatal(err)
	}
}
