package user

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"api-gateway/internal/database"
	"api-gateway/internal/models"
	"api-gateway/internal/security"
	"github.com/google/uuid"
)

var (
	ErrEmailAlreadyExists = errors.New("el correo electrónico ya está registrado.")
	ErrAdminRoleForbidden = errors.New("está estrictamente prohibido crear usuarios con rol 'admin' vía API")
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

	role := strings.ToLower(strings.TrimSpace(req.Role))
	if role == "" {
		role = models.RoleUser
	}
	if role == models.RoleAdmin {
		return nil, ErrAdminRoleForbidden
	}

	hashedPassword, err := security.HashPassword(req.Password)
	if err != nil {
		return nil, fmt.Errorf("error al hashear contraseña: %w", err)
	}

	userID := strings.TrimSpace(req.ID)
	if userID == "" {
		userID = uuid.New().String()
	}

	newUser := &models.User{
		ID:             userID,
		Email:          req.Email,
		HashedPassword: hashedPassword,
		Role:           role,
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

// CreateUserM2M crea un nuevo usuario para backends clientes (M2M)
func (s *Service) CreateUserM2M(ctx context.Context, req *models.M2MUserCreateRequest) (*models.M2MUserResponse, error) {
	existing, err := s.db.GetUserByEmail(ctx, req.Email)
	if err != nil {
		return nil, fmt.Errorf("error al verificar existencia de usuario: %w", err)
	}
	if existing != nil {
		return nil, ErrEmailAlreadyExists
	}

	role := strings.ToLower(strings.TrimSpace(req.Role))
	if role == "" {
		role = models.RoleUser
	}
	if role == models.RoleAdmin {
		return nil, ErrAdminRoleForbidden
	}

	hashedPassword, err := security.HashPassword(req.Password)
	if err != nil {
		return nil, fmt.Errorf("error al hashear contraseña: %w", err)
	}

	userID := strings.TrimSpace(req.ID)
	if userID == "" {
		userID = uuid.New().String()
	}

	newUser := &models.User{
		ID:             userID,
		Email:          req.Email,
		HashedPassword: hashedPassword,
		Role:           role,
		ClientID:       req.ClientID,
	}

	if err := s.db.CreateUser(ctx, newUser); err != nil {
		return nil, fmt.Errorf("error al registrar usuario en la base de datos: %w", err)
	}

	return &models.M2MUserResponse{
		ID:       newUser.ID,
		Email:    newUser.Email,
		Role:     newUser.Role,
		ClientID: newUser.ClientID,
	}, nil
}

// DeleteUserM2M elimina un usuario atómicamente por su UUID para operaciones de compensación/rollback
func (s *Service) DeleteUserM2M(ctx context.Context, id string) error {
	if err := s.db.DeleteUser(ctx, id); err != nil {
		return fmt.Errorf("error al eliminar usuario: %w", err)
	}
	return nil
}
