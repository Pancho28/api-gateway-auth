package database

import (
	"context"
	"fmt"
	"log"
	"time"

	"api-gateway/internal/config"
	"api-gateway/internal/models"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// UserRepository define las operaciones de acceso a datos para usuarios
type UserRepository interface {
	GetUserByEmail(ctx context.Context, email string) (*models.User, error)
	CreateUser(ctx context.Context, user *models.User) error
}

// DB envuelve el pool de conexiones de pgxpool e implementa UserRepository
type DB struct {
	Pool *pgxpool.Pool
}

// NewPostgresPool inicializa y configura el pool de conexiones de PostgreSQL
// optimizado para compatibilidad con PgBouncer en transaction mode (ADR-001)
func NewPostgresPool(ctx context.Context, cfg *config.Config) (*DB, error) {
	poolConfig, err := pgxpool.ParseConfig(cfg.DatabaseURL())
	if err != nil {
		return nil, fmt.Errorf("error al parsear configuración de BD: %w", err)
	}

	// Mitigación de Prepared Statements para PgBouncer en Transaction Mode (ADR-001)
	poolConfig.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeSimpleProtocol

	// Ajustes de capacidad del pool de conexiones
	poolConfig.MinConns = 5
	poolConfig.MaxConns = 25
	poolConfig.MaxConnLifetime = 30 * time.Minute
	poolConfig.MaxConnIdleTime = 5 * time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("error al inicializar pool de conexiones pgx: %w", err)
	}

	// Verificar conectividad mediante Ping
	pingCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		log.Printf("Advertencia: No se pudo conectar inmediatamente a PostgreSQL: %v (degradación permitida en arranque/tests)", err)
	}

	return &DB{Pool: pool}, nil
}

// InitSchema crea las tablas e índices si no existen
func (db *DB) InitSchema(ctx context.Context) error {
	query := `
	CREATE TABLE IF NOT EXISTS users (
		id VARCHAR(36) PRIMARY KEY,
		email VARCHAR(255) UNIQUE NOT NULL,
		hashed_password VARCHAR(255) NOT NULL,
		role VARCHAR(50) NOT NULL DEFAULT 'USER'
	);
	CREATE INDEX IF NOT EXISTS ix_users_email ON users(email);
	`
	_, err := db.Pool.Exec(ctx, query)
	if err != nil {
		return fmt.Errorf("error al inicializar esquema de base de datos: %w", err)
	}
	return nil
}

// GetUserByEmail busca un usuario en PostgreSQL por su dirección de correo
func (db *DB) GetUserByEmail(ctx context.Context, email string) (*models.User, error) {
	query := `SELECT id, email, hashed_password, role FROM users WHERE email = $1 LIMIT 1`
	var u models.User
	err := db.Pool.QueryRow(ctx, query, email).Scan(&u.ID, &u.Email, &u.HashedPassword, &u.Role)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("error al consultar usuario por email: %w", err)
	}
	return &u, nil
}

// CreateUser inserta un nuevo registro de usuario en PostgreSQL
func (db *DB) CreateUser(ctx context.Context, user *models.User) error {
	query := `INSERT INTO users (id, email, hashed_password, role) VALUES ($1, $2, $3, $4)`
	_, err := db.Pool.Exec(ctx, query, user.ID, user.Email, user.HashedPassword, user.Role)
	if err != nil {
		return fmt.Errorf("error al insertar usuario en la base de datos: %w", err)
	}
	return nil
}

// Close libera las conexiones del pool
func (db *DB) Close() {
	if db.Pool != nil {
		db.Pool.Close()
	}
}
