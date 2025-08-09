package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
)

type App struct{ DB *pgxpool.Pool }

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func main() {
	_ = godotenv.Load()
	port := env("APP_PORT", "8080")
	dsn := env("DB_DSN", "postgres://postgres:postgres@localhost:5432/appdb?sslmode=disable")

	ctx := context.Background()
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		log.Fatalf("parse dsn: %v", err)
	}
	cfg.MaxConns = 5

	db, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		log.Fatalf("db connect: %v", err)
	}
	defer db.Close()

	pctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	if err := db.Ping(pctx); err != nil {
		log.Fatalf("db ping: %v", err)
	}

	app := &App{DB: db}
	r := gin.Default()

	r.GET("/healthz", app.healthz)
	r.GET("/api/v1/me", app.me)

	log.Printf("listening on :%s", port)
	if err := r.Run(":" + port); err != nil {
		log.Fatal(err)
	}
}

func (a *App) healthz(c *gin.Context) {
	var now time.Time
	if err := a.DB.QueryRow(c, "SELECT NOW()").Scan(&now); err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"status": "db_unavailable", "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok", "db_time": now.UTC().Format(time.RFC3339)})
}

func (a *App) me(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"id": "demo", "name": "Eugene"})
}
