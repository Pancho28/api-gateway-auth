package user

import (
	"context"
	"errors"
	"fmt"

	"api-gateway/internal/database"
	"api-gateway/internal/models"
	"api-gateway/internal/security"
	"github.com/google/uuid"
)

var (
	ErrEmailAlreadyExists = errors.New("el correo electrónico ya está registrado.")
)

type Service struct {
	db database.UserRepository
}

func NewService(db database.UserRepository) *Service {
	return &Service{db: db}
}

// CreateUserByAdmin crea un nuevo usuario tras verificar que el email no esté duplicado
func (s *Service) CreateUserByAdmin(ctx context.Context, req *models.AdminUserCreateRequest) (*models.UserResponse, error) {
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
		return nil, fmt.Errorf("error al registrar usuario en la base de datos: %w", err)
	}

	return &models.UserResponse{
		ID:    newUser.ID,
		Email: newUser.Email,
		Role:  newUser.Role,
	}, nil
}
