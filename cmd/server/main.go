package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"pr-reviewer-service/internal/config"
	"pr-reviewer-service/internal/handlers"
	"pr-reviewer-service/internal/repository"
	"pr-reviewer-service/internal/service"
	"time"

	"github.com/gorilla/mux"
	_ "github.com/jackc/pgx/v5/stdlib" // Важно: импорт драйвера pgx
	"github.com/jmoiron/sqlx"
)

func main() {
	// Load configuration
	cfg := config.Load()

	// Database connection with retry logic
	var db *sqlx.DB
	var err error

	log.Println("Connecting to database...")
	for i := 0; i < 10; i++ {
		db, err = sqlx.Connect("pgx", cfg.DatabaseURL())
		if err == nil {
			break
		}
		log.Printf("Failed to connect to database (attempt %d/10): %v", i+1, err)
		time.Sleep(2 * time.Second)
	}
	if err != nil {
		log.Fatalf("Failed to connect to database after retries: %v", err)
	}
	defer db.Close()

	// Test database connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		log.Fatalf("Database ping failed: %v", err)
	}
	log.Println("Successfully connected to database")

	// Initialize database schema
	if err := initDatabase(db); err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	log.Println("Database schema initialized")

	// Initialize dependencies
	repo := repository.NewRepository(db)
	svc := service.NewService(repo)
	handler := handlers.NewHandlers(svc)

	// Setup routes
	r := mux.NewRouter()

	// Team endpoints
	r.HandleFunc("/team/add", handler.AddTeam).Methods("POST")
	r.HandleFunc("/team/get", handler.GetTeam).Methods("GET")

	// User endpoints
	r.HandleFunc("/users/setIsActive", handler.SetUserActive).Methods("POST")
	r.HandleFunc("/users/getReview", handler.GetUserReviewPullRequests).Methods("GET")

	// PR endpoints
	r.HandleFunc("/pullRequest/create", handler.CreatePullRequest).Methods("POST")
	r.HandleFunc("/pullRequest/merge", handler.MergePullRequest).Methods("POST")
	r.HandleFunc("/pullRequest/reassign", handler.ReassignReviewer).Methods("POST")

	// Health check
	r.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		// Check database connection
		if err := db.PingContext(r.Context()); err != nil {
			http.Error(w, `{"status":"database error"}`, http.StatusServiceUnavailable)
			return
		}

		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}).Methods("GET")

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Server starting on port %s", port)
	log.Fatal(http.ListenAndServe(":"+port, r))
}

func initDatabase(db *sqlx.DB) error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS teams (
            team_name VARCHAR(255) PRIMARY KEY,
            created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
        )`,

		`CREATE TABLE IF NOT EXISTS users (
            user_id VARCHAR(255) PRIMARY KEY,
            username VARCHAR(255) NOT NULL,
            team_name VARCHAR(255) NOT NULL REFERENCES teams(team_name) ON DELETE CASCADE,
            is_active BOOLEAN DEFAULT true,
            created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
            updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
        )`,

		`CREATE TABLE IF NOT EXISTS pull_requests (
            pull_request_id VARCHAR(255) PRIMARY KEY,
            pull_request_name VARCHAR(255) NOT NULL,
            author_id VARCHAR(255) NOT NULL REFERENCES users(user_id),
            status VARCHAR(50) NOT NULL DEFAULT 'OPEN',
            assigned_reviewers JSONB NOT NULL DEFAULT '[]',
            created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
            merged_at TIMESTAMP NULL,
            updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
        )`,

		`CREATE INDEX IF NOT EXISTS idx_users_team_active ON users(team_name, is_active)`,
		`CREATE INDEX IF NOT EXISTS idx_pr_author ON pull_requests(author_id)`,
		`CREATE INDEX IF NOT EXISTS idx_pr_status ON pull_requests(status)`,
	}

	for _, query := range queries {
		if _, err := db.Exec(query); err != nil {
			return err
		}
	}
	return nil
}
