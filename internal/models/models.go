package models

import (
	"time"
)

type User struct {
	UserID   string `json:"user_id" db:"user_id"`
	Username string `json:"username" db:"username"`
	TeamName string `json:"team_name" db:"team_name"`
	IsActive bool   `json:"is_active" db:"is_active"`
}

type Team struct {
	TeamName string `json:"team_name" db:"team_name"`
	Members  []User `json:"members"`
}

type PullRequest struct {
	PullRequestID     string     `json:"pull_request_id" db:"pull_request_id"`
	PullRequestName   string     `json:"pull_request_name" db:"pull_request_name"`
	AuthorID          string     `json:"author_id" db:"author_id"`
	Status            string     `json:"status" db:"status"`
	AssignedReviewers []string   `json:"assigned_reviewers" db:"assigned_reviewers"`
	CreatedAt         time.Time  `json:"createdAt" db:"created_at"`
	MergedAt          *time.Time `json:"mergedAt" db:"merged_at"`
}

type PullRequestShort struct {
	PullRequestID   string `json:"pull_request_id"`
	PullRequestName string `json:"pull_request_name"`
	AuthorID        string `json:"author_id"`
	Status          string `json:"status"`
}
