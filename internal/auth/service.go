package auth

import (
	"database/sql"
	"errors"
	"fmt"

	"golang.org/x/crypto/bcrypt"

	"github.com/priyansx01/corn-hub-clone/internal/domain"
)

var (
	ErrInvalidCredentials = errors.New("invalid email or password")
	ErrUserNotFound       = errors.New("user not found")
)

// Service handles authentication business logic.
type Service struct {
	db  *sql.DB
	jwt *JWTManager
}

// NewService creates a new auth service.
func NewService(db *sql.DB, jwt *JWTManager) *Service {
	return &Service{db: db, jwt: jwt}
}

// LoginRequest is the payload for POST /auth/login.
type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// LoginResponse is returned on successful authentication.
type LoginResponse struct {
	AccessToken  string       `json:"access_token"`
	RefreshToken string       `json:"refresh_token"`
	User         *domain.User `json:"user"`
}

// Login authenticates a user by email + password and returns JWT tokens.
func (s *Service) Login(req LoginRequest) (*LoginResponse, error) {
	if req.Email == "" || req.Password == "" {
		return nil, ErrInvalidCredentials
	}

	user, err := s.findUserByEmail(req.Email)
	if err != nil {
		return nil, ErrInvalidCredentials
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		return nil, ErrInvalidCredentials
	}

	accessToken, err := s.jwt.GenerateAccessToken(user)
	if err != nil {
		return nil, fmt.Errorf("generate access token: %w", err)
	}

	refreshToken, err := s.jwt.GenerateRefreshToken(user.ID)
	if err != nil {
		return nil, fmt.Errorf("generate refresh token: %w", err)
	}

	return &LoginResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		User:         user,
	}, nil
}

// RefreshRequest is the payload for POST /auth/refresh.
type RefreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

// Refresh validates a refresh token and returns a new access token.
func (s *Service) Refresh(req RefreshRequest) (string, error) {
	userID, err := s.jwt.ValidateRefreshToken(req.RefreshToken)
	if err != nil {
		return "", err
	}

	user, err := s.findUserByID(userID)
	if err != nil {
		return "", ErrUserNotFound
	}

	return s.jwt.GenerateAccessToken(user)
}

// GetUserByID returns a user by their LMS user ID.
func (s *Service) GetUserByID(id string) (*domain.User, error) {
	return s.findUserByID(id)
}

// ─── Repository ───────────────────────────────────────────────────────────────

func (s *Service) findUserByEmail(email string) (*domain.User, error) {
	var user domain.User
	err := s.db.QueryRow(`
		SELECT id, smartfm_id, name, email, password_hash, role, avatar_url, created_at, updated_at
		FROM users WHERE email = $1
	`, email).Scan(
		&user.ID, &user.SmartFMID, &user.Name, &user.Email,
		&user.Password, &user.Role, &user.AvatarURL,
		&user.CreatedAt, &user.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("query user by email: %w", err)
	}
	return &user, nil
}

func (s *Service) findUserByID(id string) (*domain.User, error) {
	var user domain.User
	err := s.db.QueryRow(`
		SELECT id, smartfm_id, name, email, password_hash, role, avatar_url, created_at, updated_at
		FROM users WHERE id = $1
	`, id).Scan(
		&user.ID, &user.SmartFMID, &user.Name, &user.Email,
		&user.Password, &user.Role, &user.AvatarURL,
		&user.CreatedAt, &user.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("query user by id: %w", err)
	}
	return &user, nil
}
