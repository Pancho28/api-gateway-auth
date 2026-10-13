package config

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

// ServiceConfig define la URL y API Key de un microservicio aguas abajo
type ServiceConfig struct {
	URL    string `json:"url"`
	APIKey string `json:"api_key"`
}

// Config contiene todas las variables de configuración de la aplicación
type Config struct {
	Port                     string
	SecretKey                string
	Algorithm                string
	AccessTokenExpireMinutes int

	// PostgreSQL
	DBUser     string
	DBPassword string
	DBHost     string
	DBPort     int
	DBName     string

	// Admin inicial
	EmailAdmin    string
	PasswordAdmin string

	// CORS
	Origin string

	// Redis
	RedisHost     string
	RedisPort     int
	RedisDB       int
	RedisPassword string
	RedisCacheTTL time.Duration

	// Microservicios: prefix -> ServiceConfig
	Microservices map[string]ServiceConfig
}

// DatabaseURL genera la cadena de conexión compatible con pgxpool y PgBouncer
func (c *Config) DatabaseURL() string {
	return fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=disable",
		c.DBUser, c.DBPassword, c.DBHost, c.DBPort, c.DBName)
}

// RedisURL genera la dirección de conexión para Redis
func (c *Config) RedisAddr() string {
	return fmt.Sprintf("%s:%d", c.RedisHost, c.RedisPort)
}

// Load lee las variables de entorno desde .env (si existe) y las variables del sistema
func Load() (*Config, error) {
	// Intentar cargar .env desde el directorio de trabajo o directorio raíz
	_ = godotenv.Load()
	_ = godotenv.Load("../.env")

	cfg := &Config{
		Port:                     getEnv("PORT", "8000"),
		SecretKey:                getEnv("SECRET_KEY", ""),
		Algorithm:                getEnv("ALGORITHM", "HS256"),
		AccessTokenExpireMinutes: getEnvInt("ACCESS_TOKEN_EXPIRE_MINUTES", 30),

		DBUser:     getEnv("DB_USER", "postgres"),
		DBPassword: getEnv("DB_PASSWORD", "postgres"),
		DBHost:     getEnv("DB_HOST", "localhost"),
		DBPort:     getEnvInt("DB_PORT", 5432),
		DBName:     getEnv("DB_NAME", "gateway_auth"),

		EmailAdmin:    getEnv("EMAIL_ADMIN", "admin@admin.com"),
		PasswordAdmin: getEnv("PASSWORD_ADMIN", "Admin1234"),

		Origin: getEnv("ORIGIN", ""),

		RedisHost:     getEnv("REDIS_HOST", "localhost"),
		RedisPort:     getEnvInt("REDIS_PORT", 6379),
		RedisDB:       getEnvInt("REDIS_DB", 0),
		RedisPassword: getEnv("REDIS_PASSWORD", ""),
		RedisCacheTTL: time.Duration(getEnvInt("REDIS_CACHE_TTL", 60)) * time.Second,

		Microservices: make(map[string]ServiceConfig),
	}

	rawServices := getEnv("MICROSERVICES", "")
	if rawServices != "" {
		var services map[string]ServiceConfig
		if err := json.Unmarshal([]byte(rawServices), &services); err != nil {
			return nil, fmt.Errorf("error al parsear MICROSERVICES: %w", err)
		}
		cfg.Microservices = services
	}

	return cfg, nil
}

func getEnv(key, defaultVal string) string {
	if val, ok := os.LookupEnv(key); ok && val != "" {
		return val
	}
	return defaultVal
}

func getEnvInt(key string, defaultVal int) int {
	if val, ok := os.LookupEnv(key); ok && val != "" {
		if intVal, err := strconv.Atoi(val); err == nil {
			return intVal
		}
	}
	return defaultVal
}
