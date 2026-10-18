package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"api-gateway/internal/config"
	"api-gateway/internal/database"
	"api-gateway/internal/models"
	"api-gateway/internal/redis"
	"api-gateway/internal/security"
	"github.com/google/uuid"
)

var (
	ErrInvalidCredentials = errors.New("Correo o contraseña incorrectos")
	ErrEmailAlreadyExists = errors.New("el correo electrónico ya está registrado.")
)


type Service struct {
	db    database.UserRepository
	redis *redis.Client
	cfg   *config.Config
}

func NewService(db database.UserRepository, rdb *redis.Client, cfg *config.Config) *Service {
	return &Service{
		db:    db,
		redis: rdb,
		cfg:   cfg,
	}
}

// Login autentica a un usuario contra PostgreSQL y cachea su sesión en Redis
func (s *Service) Login(ctx context.Context, req *models.UserLoginRequest) (*models.TokenResponse, error) {
	user, err := s.db.GetUserByEmail(ctx, req.Email)
	if err != nil {
		return nil, fmt.Errorf("error al buscar usuario: %w", err)
	}

	if user == nil || !security.VerifyPassword(req.Password, user.HashedPassword) {
		return nil, ErrInvalidCredentials
	}

	expireDuration := time.Duration(s.cfg.AccessTokenExpireMinutes) * time.Minute
	tokenStr, err := security.GenerateToken(user.Email, user.Role, req.ClientID, s.cfg.SecretKey, expireDuration)
	if err != nil {
		return nil, fmt.Errorf("error al generar token de acceso: %w", err)
	}

	// Almacenar sesión en Redis con TTL igual a la expiración del JWT (ADR-002)
	sessionKey := fmt.Sprintf("session:%s", user.Email)
	sessionData := models.SessionData{
		ID:       user.ID,
		Email:    user.Email,
		Role:     user.Role,
		ClientID: req.ClientID,
	}
	s.redis.SetJSON(ctx, sessionKey, sessionData, expireDuration)

	return &models.TokenResponse{
		User:     user.Email,
		Role:     user.Role,
		Token:    tokenStr,
		ClientID: req.ClientID,
	}, nil
}

// Register registra un nuevo usuario, persiste sus credenciales y genera un token JWT
func (s *Service) Register(ctx context.Context, req *models.UserRegisterRequest) (*models.TokenResponse, error) {
	existing, err := s.db.GetUserByEmail(ctx, req.Email)
	if err != nil {
		return nil, fmt.Errorf("error al verificar existencia de usuario: %w", err)
	}
	if existing != nil {
		return nil, ErrEmailAlreadyExists
	}

	hashedPassword, err := security.HashPassword(req.Password)
	if err != nil {
		return nil, fmt.Errorf("error al hashear contraseña: %w", err)
	}

	newUser := &models.User{
		ID:             uuid.New().String(),
		Email:          req.Email,
		HashedPassword: hashedPassword,
		Role:           req.Role,
	}

	if err := s.db.CreateUser(ctx, newUser); err != nil {
		return nil, fmt.Errorf("error al registrar usuario: %w", err)
	}

	expireDuration := time.Duration(s.cfg.AccessTokenExpireMinutes) * time.Minute
	tokenStr, err := security.GenerateToken(newUser.Email, newUser.Role, req.ClientID, s.cfg.SecretKey, expireDuration)
	if err != nil {
		return nil, fmt.Errorf("error al generar token de acceso: %w", err)
	}

	// Almacenar sesión en Redis con TTL igual a la expiración del JWT
	sessionKey := fmt.Sprintf("session:%s", newUser.Email)
	sessionData := models.SessionData{
		ID:       newUser.ID,
		Email:    newUser.Email,
		Role:     newUser.Role,
		ClientID: req.ClientID,
	}
	s.redis.SetJSON(ctx, sessionKey, sessionData, expireDuration)

	return &models.TokenResponse{
		User:     newUser.Email,
		Role:     newUser.Role,
		Token:    tokenStr,
		ClientID: req.ClientID,
	}, nil
}

