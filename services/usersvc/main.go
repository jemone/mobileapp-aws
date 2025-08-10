package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	oidc "github.com/coreos/go-oidc/v3/oidc"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
)

type App struct {
	DB       *pgxpool.Pool
	Verifier *oidc.IDTokenVerifier
	Issuer   string
	ClientID string
}

type User struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

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
	issuer := env("OIDC_ISSUER", "")
	clientID := env("OIDC_AUDIENCE", "")

	ctx := context.Background()

	// DB
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

	// OIDC provider + verifier (если настроен issuer)
	var verifier *oidc.IDTokenVerifier
	if issuer != "" && clientID != "" {
		var provider *oidc.Provider
		var err error
		for i := 0; i < 30; i++ {
			provider, err = oidc.NewProvider(ctx, issuer)
			if err == nil {
				break
			}
			log.Printf("oidc provider not ready (%v), retrying...", err)
			time.Sleep(2 * time.Second)
		}
		if err != nil {
			log.Fatalf("oidc provider: %v", err)
		}
		verifier = provider.Verifier(&oidc.Config{ClientID: clientID})
		log.Printf("OIDC enabled: issuer=%s clientID=%s", issuer, clientID)
	} else {
		log.Printf("OIDC disabled (missing OIDC_ISSUER or OIDC_AUDIENCE)")
	}

	app := &App{DB: db, Verifier: verifier, Issuer: issuer, ClientID: clientID}
	r := gin.Default()
	r.Use(RequestID())
	r.Use(CORS())

	// health
	r.GET("/healthz", app.healthz)
	r.GET("/api/v1/me", app.me)

	// users CRUD (пока без аутентификации)
	r.POST("/api/v1/users", app.createUser)
	r.GET("/api/v1/users", app.listUsers)
	r.GET("/api/v1/users/:id", app.getUser)
	r.DELETE("/api/v1/users/:id", app.deleteUser)

	// защищённый эндпоинт (нужен Bearer токен Keycloak)
	auth := r.Group("/api/v1", app.authMiddleware())
	auth.GET("/profile", app.profile)

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

type createUserReq struct {
	Email string `json:"email" binding:"required,email"`
	Name  string `json:"name"  binding:"required"`
}

func (a *App) createUser(c *gin.Context) {
	var in createUserReq
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_payload", "details": err.Error()})
		return
	}
	var u User
	err := a.DB.QueryRow(
		c,
		`INSERT INTO app_user (email, name) VALUES ($1, $2)
		 RETURNING id::text, email, name, created_at`,
		in.Email, in.Name,
	).Scan(&u.ID, &u.Email, &u.Name, &u.CreatedAt)
	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "insert_failed", "details": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, u)
}

func (a *App) listUsers(c *gin.Context) {
	limit := 100
	if ls := c.Query("limit"); ls != "" {
		if v, err := strconv.Atoi(ls); err == nil && v > 0 && v <= 1000 {
			limit = v
		}
	}
	rows, err := a.DB.Query(c,
		`SELECT id::text, email, name, created_at
		   FROM app_user
		   ORDER BY created_at DESC
		   LIMIT $1`, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "query_failed", "details": err.Error()})
		return
	}
	defer rows.Close()

	out := make([]User, 0, limit)
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.ID, &u.Email, &u.Name, &u.CreatedAt); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "scan_failed", "details": err.Error()})
			return
		}
		out = append(out, u)
	}
	c.JSON(http.StatusOK, out)
}

func (a *App) getUser(c *gin.Context) {
	id := c.Param("id")
	var u User
	err := a.DB.QueryRow(
		c,
		`SELECT id::text, email, name, created_at FROM app_user WHERE id = $1`,
		id,
	).Scan(&u.ID, &u.Email, &u.Name, &u.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{"error": "not_found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "query_failed", "details": err.Error()})
		return
	}
	c.JSON(http.StatusOK, u)
}

func (a *App) deleteUser(c *gin.Context) {
	id := c.Param("id")
	ct, err := a.DB.Exec(c, `DELETE FROM app_user WHERE id = $1`, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "delete_failed", "details": err.Error()})
		return
	}
	if ct.RowsAffected() == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "not_found"})
		return
	}
	c.Status(http.StatusNoContent)
}

// ===== Auth (JWT via OIDC) =====

func (a *App) authMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if a.Verifier == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "oidc_not_configured"})
			c.Abort()
			return
		}
		authz := c.GetHeader("Authorization")
		if !strings.HasPrefix(strings.ToLower(authz), "bearer ") {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "missing_bearer"})
			c.Abort()
			return
		}
		raw := strings.TrimSpace(authz[len("Bearer "):])
		idToken, err := a.Verifier.Verify(c, raw)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid_token"})
			c.Abort()
			return
		}
		var claims struct {
			Email string `json:"email"`
			Sub   string `json:"sub"`
			Name  string `json:"name"`
		}
		_ = idToken.Claims(&claims)
		// сохраним в контекст
		c.Set("sub", claims.Sub)
		c.Set("email", claims.Email)
		c.Set("name", claims.Name)
		c.Next()
	}
}

func (a *App) profile(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"sub":   c.GetString("sub"),
		"email": c.GetString("email"),
		"name":  c.GetString("name"),
	})
}

// RequestID — берём X-Request-ID или генерим новый. Кладём в контекст и ответ.
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		reqID := c.GetHeader("X-Request-ID")
		if reqID == "" {
			reqID = uuid.NewString()
		}
		c.Writer.Header().Set("X-Request-ID", reqID)
		c.Set("request_id", reqID)
		c.Next()
	}
}

// CORS — dev-режим: разрешаем все origins/headers/methods (на проде — сузим).
func CORS() gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if origin == "" {
			origin = "*"
		}
		c.Writer.Header().Set("Access-Control-Allow-Origin", origin)
		c.Writer.Header().Set("Vary", "Origin")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET,POST,PUT,PATCH,DELETE,OPTIONS")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Authorization,Content-Type,X-Request-ID")
		c.Writer.Header().Set("Access-Control-Expose-Headers", "X-Request-ID")
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}
