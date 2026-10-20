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
		log.Println("[SEED] El usuario administrador inicial ya se encuentra configurado en el sistema.")
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
		Role:           models.RoleAdmin,
	}

	if err := db.CreateUser(ctx, adminUser); err != nil {
		return fmt.Errorf("error al insertar administrador inicial: %w", err)
	}

	log.Println("[SEED] Usuario administrador inicial creado y aprovisionado exitosamente.")
	return nil
}
