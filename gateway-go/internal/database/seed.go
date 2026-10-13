package database

import (
	"context"
	"fmt"
	"log"

	"api-gateway/internal/config"
	"api-gateway/internal/models"
	"api-gateway/internal/security"
	"github.com/google/uuid"
)

// InitializeAdmin verifica si el usuario administrador existe en la base de datos.
// Si no existe, lo crea a partir de las credenciales configuradas en las variables de entorno.
func InitializeAdmin(ctx context.Context, db UserRepository, cfg *config.Config) error {
	existingUser, err := db.GetUserByEmail(ctx, cfg.EmailAdmin)
	if err != nil {
		return fmt.Errorf("error al verificar existencia del administrador: %w", err)
	}

	if existingUser != nil {
		log.Printf("[SEED] El administrador ya existe en el sistema: %s", cfg.EmailAdmin)
		return nil
	}

	hashedPassword, err := security.HashPassword(cfg.PasswordAdmin)
	if err != nil {
		return fmt.Errorf("error al hashear contraseña del administrador: %w", err)
	}

	adminUser := &models.User{
		ID:             uuid.New().String(),
		Email:          cfg.EmailAdmin,
		HashedPassword: hashedPassword,
		Role:           "ADMIN",
	}

	if err := db.CreateUser(ctx, adminUser); err != nil {
		return fmt.Errorf("error al insertar administrador inicial: %w", err)
	}

	log.Printf("[SEED] Administrador inicial creado exitosamente: %s (Rol: ADMIN)", cfg.EmailAdmin)
	return nil
}
