package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"api-gateway/internal/config"
	"api-gateway/internal/database"
	"api-gateway/internal/models"
	"api-gateway/internal/redis"
	"api-gateway/internal/security"
	"github.com/google/uuid"
)

var (
	ErrInvalidCredentials   = errors.New("Correo o contraseña incorrectos")
	ErrEmailAlreadyExists   = errors.New("el correo electrónico ya está registrado.")
	ErrTokenBindingMismatch = errors.New("contexto de seguridad inconsistente")
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
	tokenStr, err := security.GenerateToken(user.ID, user.Email, user.Role, req.ClientID, s.cfg.SecretKey, expireDuration)
	if err != nil {
		return nil, fmt.Errorf("error al generar token de acceso: %w", err)
	}

	s.cacheSession(ctx, user, req.ClientID, expireDuration)

	return &models.TokenResponse{
		ID:       user.ID,
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

	userID := strings.TrimSpace(req.ID)
	if userID == "" {
		userID = uuid.New().String()
	}

	role := strings.ToLower(strings.TrimSpace(req.Role))
	if role == "" {
		role = models.RoleUser
	}

	newUser := &models.User{
		ID:             userID,
		Email:          req.Email,
		HashedPassword: hashedPassword,
		Role:           role,
	}

	if err := s.db.CreateUser(ctx, newUser); err != nil {
		return nil, fmt.Errorf("error al registrar usuario: %w", err)
	}

	expireDuration := time.Duration(s.cfg.AccessTokenExpireMinutes) * time.Minute
	tokenStr, err := security.GenerateToken(newUser.ID, newUser.Email, newUser.Role, req.ClientID, s.cfg.SecretKey, expireDuration)
	if err != nil {
		return nil, fmt.Errorf("error al generar token de acceso: %w", err)
	}

	s.cacheSession(ctx, newUser, req.ClientID, expireDuration)

	return &models.TokenResponse{
		ID:       newUser.ID,
		User:     newUser.Email,
		Role:     newUser.Role,
		Token:    tokenStr,
		ClientID: req.ClientID,
	}, nil
}

// Logout invalida la sesión del usuario en Redis y registra el jti del JWT en la denylist
func (s *Service) Logout(ctx context.Context, tokenStr, reqClientID string) error {
	claims, err := security.ValidateToken(tokenStr, s.cfg.SecretKey)
	if err != nil {
		return ErrInvalidCredentials
	}

	reqClientID = strings.TrimSpace(reqClientID)
	if reqClientID != "" && claims.ClientID != "" && reqClientID != claims.ClientID {
		return ErrTokenBindingMismatch
	}

	if claims.RegisteredClaims.ID != "" && claims.ExpiresAt != nil {
		remainingTTL := time.Until(claims.ExpiresAt.Time)
		if remainingTTL > 0 {
			s.redis.SetString(ctx, redis.RevokedJWTKey(claims.RegisteredClaims.ID), "1", remainingTTL)
		}
	}

	s.redis.Delete(ctx, redis.SessionKey(claims.Subject, claims.ClientID))
	if claims.ClientID != "" {
		// Limpieza defensiva por compatibilidad con claves heredadas sin client_id
		s.redis.Delete(ctx, redis.SessionKey(claims.Subject, ""))
	}
	return nil
}

func (s *Service) cacheSession(ctx context.Context, user *models.User, clientID string, ttl time.Duration) {
	sessionKey := redis.SessionKey(user.Email, clientID)
	sessionData := models.SessionData{
		ID:       user.ID,
		Email:    user.Email,
		Role:     user.Role,
		ClientID: clientID,
	}
	s.redis.SetJSON(ctx, sessionKey, sessionData, ttl)
}
