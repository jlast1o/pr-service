package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"pr-reviewer-service/internal/models"

	"github.com/jmoiron/sqlx"
)

type Repository struct {
	db *sqlx.DB
}

func NewRepository(db *sqlx.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) CreateTeam(ctx context.Context, team *models.Team) error {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Check if team already exists
	var existingTeam string
	err = tx.GetContext(ctx, &existingTeam,
		"SELECT team_name FROM teams WHERE team_name = $1", team.TeamName)
	if err == nil {
		return fmt.Errorf("team already exists")
	} else if !errors.Is(err, sql.ErrNoRows) {
		return err
	}

	// Insert team
	_, err = tx.ExecContext(ctx,
		"INSERT INTO teams (team_name) VALUES ($1)", team.TeamName)
	if err != nil {
		return err
	}

	// Insert/update users
	for _, member := range team.Members {
		_, err = tx.ExecContext(ctx, `
			INSERT INTO users (user_id, username, team_name, is_active) 
			VALUES ($1, $2, $3, $4)
			ON CONFLICT (user_id) 
			DO UPDATE SET username = $2, team_name = $3, is_active = $4`,
			member.UserID, member.Username, team.TeamName, member.IsActive)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (r *Repository) GetUserByID(ctx context.Context, userID string) (models.User, error) {
	var user models.User
	err := r.db.GetContext(ctx, &user,
		"SELECT user_id, username, team_name, is_active FROM users WHERE user_id = $1", userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return user, fmt.Errorf("user not found")
		}
		return user, err
	}
	return user, nil
}

func (r *Repository) GetTeam(ctx context.Context, teamName string) (*models.Team, error) {
	var team models.Team
	err := r.db.GetContext(ctx, &team,
		"SELECT team_name FROM teams WHERE team_name = $1", teamName)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("team not found")
		}
		return nil, err
	}

	var members []models.User
	err = r.db.SelectContext(ctx, &members,
		"SELECT user_id, username, team_name, is_active FROM users WHERE team_name = $1", teamName)
	if err != nil {
		return nil, err
	}

	team.Members = members
	return &team, nil
}

func (r *Repository) UpdateUserActivity(ctx context.Context, userID string, isActive bool) (*models.User, error) {
	var user models.User
	err := r.db.GetContext(ctx, &user, `
		UPDATE users SET is_active = $1 
		WHERE user_id = $2 
		RETURNING user_id, username, team_name, is_active`,
		isActive, userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("user not found")
		}
		return nil, err
	}
	return &user, nil
}

func (r *Repository) CreatePullRequest(ctx context.Context, pr *models.PullRequest) error {
	reviewersJSON, err := json.Marshal(pr.AssignedReviewers)
	if err != nil {
		return err
	}

	_, err = r.db.ExecContext(ctx, `
		INSERT INTO pull_requests 
		(pull_request_id, pull_request_name, author_id, status, assigned_reviewers, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)`,
		pr.PullRequestID, pr.PullRequestName, pr.AuthorID, pr.Status, reviewersJSON, pr.CreatedAt)

	if err != nil {
		return fmt.Errorf("PR id already exists")
	}
	return nil
}

func (r *Repository) GetPullRequest(ctx context.Context, prID string) (*models.PullRequest, error) {
	var pr models.PullRequest
	var reviewersJSON string

	err := r.db.GetContext(ctx, &pr, `
		SELECT pull_request_id, pull_request_name, author_id, status, assigned_reviewers, created_at, merged_at
		FROM pull_requests WHERE pull_request_id = $1`, prID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("PR not found")
		}
		return nil, err
	}

	err = json.Unmarshal([]byte(reviewersJSON), &pr.AssignedReviewers)
	if err != nil {
		return nil, err
	}

	return &pr, nil
}

func (r *Repository) UpdatePullRequest(ctx context.Context, pr *models.PullRequest) error {
	reviewersJSON, err := json.Marshal(pr.AssignedReviewers)
	if err != nil {
		return err
	}

	_, err = r.db.ExecContext(ctx, `
		UPDATE pull_requests 
		SET status = $1, assigned_reviewers = $2, merged_at = $3
		WHERE pull_request_id = $4`,
		pr.Status, reviewersJSON, pr.MergedAt, pr.PullRequestID)
	return err
}

func (r *Repository) GetActiveTeamMembers(ctx context.Context, teamName string, excludeUserID string) ([]models.User, error) {
	var users []models.User
	query := `
		SELECT user_id, username, team_name, is_active 
		FROM users 
		WHERE team_name = $1 AND is_active = true AND user_id != $2`

	err := r.db.SelectContext(ctx, &users, query, teamName, excludeUserID)
	return users, err
}

func (r *Repository) GetUserReviewPullRequests(ctx context.Context, userID string) ([]models.PullRequestShort, error) {
	var prs []models.PullRequestShort

	query := `
		SELECT DISTINCT pr.pull_request_id, pr.pull_request_name, pr.author_id, pr.status
		FROM pull_requests pr
		WHERE $1 = ANY(pr.assigned_reviewers)`

	err := r.db.SelectContext(ctx, &prs, query, userID)
	return prs, err
}
