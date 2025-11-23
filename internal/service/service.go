package service

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"pr-reviewer-service/internal/models"
	"pr-reviewer-service/internal/repository"
	"time"
)

type Service struct {
	repo *repository.Repository
}

func NewService(repo *repository.Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) CreateTeam(ctx context.Context, team *models.Team) error {
	return s.repo.CreateTeam(ctx, team)
}

func (s *Service) GetTeam(ctx context.Context, teamName string) (*models.Team, error) {
	return s.repo.GetTeam(ctx, teamName)
}

func (s *Service) UpdateUserActivity(ctx context.Context, userID string, isActive bool) (*models.User, error) {
	return s.repo.UpdateUserActivity(ctx, userID, isActive)
}

func (s *Service) CreatePullRequest(ctx context.Context, prCreate *models.PullRequest) (*models.PullRequest, error) {
	// Get author info to find team
	var author models.User
	// This would need a method to get user by ID - adding to repository
	author, err := s.repo.GetUserByID(ctx, prCreate.AuthorID)
	if err != nil {
		return nil, fmt.Errorf("author not found")
	}

	// Get active team members excluding author
	teamMembers, err := s.repo.GetActiveTeamMembers(ctx, author.TeamName, prCreate.AuthorID)
	if err != nil {
		return nil, err
	}

	// Select up to 2 random reviewers
	reviewers := s.selectRandomReviewers(teamMembers, 2)
	reviewerIDs := make([]string, len(reviewers))
	for i, reviewer := range reviewers {
		reviewerIDs[i] = reviewer.UserID
	}

	pr := &models.PullRequest{
		PullRequestID:     prCreate.PullRequestID,
		PullRequestName:   prCreate.PullRequestName,
		AuthorID:          prCreate.AuthorID,
		Status:            "OPEN",
		AssignedReviewers: reviewerIDs,
		CreatedAt:         time.Now(),
	}

	err = s.repo.CreatePullRequest(ctx, pr)
	if err != nil {
		return nil, err
	}

	return pr, nil
}

func (s *Service) selectRandomReviewers(users []models.User, max int) []models.User {
	if len(users) == 0 {
		return []models.User{}
	}

	if len(users) <= max {
		return users
	}

	// Shuffle and select
	shuffled := make([]models.User, len(users))
	copy(shuffled, users)
	rand.Shuffle(len(shuffled), func(i, j int) {
		shuffled[i], shuffled[j] = shuffled[j], shuffled[i]
	})

	return shuffled[:max]
}

func (s *Service) MergePullRequest(ctx context.Context, prID string) (*models.PullRequest, error) {
	pr, err := s.repo.GetPullRequest(ctx, prID)
	if err != nil {
		return nil, err
	}

	if pr.Status == "MERGED" {
		return pr, nil // Idempotent
	}

	pr.Status = "MERGED"
	now := time.Now()
	pr.MergedAt = &now

	err = s.repo.UpdatePullRequest(ctx, pr)
	if err != nil {
		return nil, err
	}

	return pr, nil
}

func (s *Service) ReassignReviewer(ctx context.Context, prID string, oldUserID string) (*models.PullRequest, string, error) {
	pr, err := s.repo.GetPullRequest(ctx, prID)
	if err != nil {
		return nil, "", err
	}

	if pr.Status == "MERGED" {
		return nil, "", errors.New("cannot reassign on merged PR")
	}

	// Check if old user is assigned
	found := false
	for _, reviewer := range pr.AssignedReviewers {
		if reviewer == oldUserID {
			found = true
			break
		}
	}
	if !found {
		return nil, "", errors.New("reviewer is not assigned to this PR")
	}

	// Get old user's team
	oldUser, err := s.repo.GetUserByID(ctx, oldUserID)
	if err != nil {
		return nil, "", fmt.Errorf("old reviewer not found")
	}

	// Get available candidates from the same team
	candidates, err := s.repo.GetActiveTeamMembers(ctx, oldUser.TeamName, oldUserID)
	if err != nil {
		return nil, "", err
	}

	// Remove candidates already assigned to this PR
	availableCandidates := []models.User{}
	for _, candidate := range candidates {
		alreadyAssigned := false
		for _, reviewer := range pr.AssignedReviewers {
			if reviewer == candidate.UserID {
				alreadyAssigned = true
				break
			}
		}
		if !alreadyAssigned {
			availableCandidates = append(availableCandidates, candidate)
		}
	}

	if len(availableCandidates) == 0 {
		return nil, "", errors.New("no active replacement candidate in team")
	}

	// Select random candidate
	newReviewer := availableCandidates[rand.Intn(len(availableCandidates))]

	// Replace reviewer
	for i, reviewer := range pr.AssignedReviewers {
		if reviewer == oldUserID {
			pr.AssignedReviewers[i] = newReviewer.UserID
			break
		}
	}

	err = s.repo.UpdatePullRequest(ctx, pr)
	if err != nil {
		return nil, "", err
	}

	return pr, newReviewer.UserID, nil
}

func (s *Service) GetUserReviewPullRequests(ctx context.Context, userID string) ([]models.PullRequestShort, error) {
	return s.repo.GetUserReviewPullRequests(ctx, userID)
}
