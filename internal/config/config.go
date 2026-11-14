package config

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

// DefaultServiceTimeoutSeconds define el tiempo de espera por defecto hacia un microservicio aguas abajo
const DefaultServiceTimeoutSeconds = 15

// ServiceConfig define la URL, API Key y reglas de aislamiento de un microservicio aguas abajo
type ServiceConfig struct {
	URL                string   `json:"url"`
	APIKey             string   `json:"api_key"`
	AllowedClients     []string `json:"allowed_clients"`
	AllowedAudiences   []string `json:"allowed_audiences,omitempty"`
	RequiredRoles      []string `json:"required_roles,omitempty"`
	PublicPaths        []string `json:"public_paths,omitempty"`
	AllowAnonymous     bool     `json:"allow_anonymous,omitempty"`
	PassthroughHeaders []string `json:"passthrough_headers,omitempty"`
	TimeoutSeconds     int      `json:"timeout_seconds,omitempty"`
}

// Timeout retorna la duración máxima configurada para el microservicio o el valor por defecto (15s)
func (s ServiceConfig) Timeout() time.Duration {
	if s.TimeoutSeconds <= 0 {
		return time.Duration(DefaultServiceTimeoutSeconds) * time.Second
	}
	return time.Duration(s.TimeoutSeconds) * time.Second
}

// IsHeaderPassthroughAllowed verifica si una cabecera HTTP personalizada está en la lista blanca del microservicio
func (s *ServiceConfig) IsHeaderPassthroughAllowed(headerName string) bool {
	if len(s.PassthroughHeaders) == 0 {
		return false
	}
	cleanName := strings.ToLower(strings.TrimSpace(headerName))
	for _, h := range s.PassthroughHeaders {
		if strings.ToLower(strings.TrimSpace(h)) == cleanName {
			return true
		}
	}
	return false
}

// IsPathPublic determina si una sub-ruta del microservicio permite acceso público/anónimo
func (s *ServiceConfig) IsPathPublic(subPath string) bool {
	if s.AllowAnonymous {
		return true
	}
	if len(s.PublicPaths) == 0 {
		return false
	}
	cleanSubPath := "/" + strings.Trim(subPath, "/")
	for _, p := range s.PublicPaths {
		cleanP := strings.TrimSpace(p)
		if cleanP == "*" || cleanP == "/*" {
			return true
		}
		cleanP = "/" + strings.Trim(cleanP, "/")
		if cleanSubPath == cleanP || strings.HasPrefix(cleanSubPath, cleanP+"/") || cleanP == "/" {
			return true
		}
	}
	return false
}

// IsClientAllowed verifica si un client_id tiene acceso según la lista blanca configurada
func (s *ServiceConfig) IsClientAllowed(clientID string) bool {
	if len(s.AllowedClients) == 0 {
		return false // Política Zero Trust / Deny-by-default: sin clientes configurados se bloquea
	}
	for _, c := range s.AllowedClients {
		if c == "*" || (clientID != "" && c == clientID) {
			return true
		}
	}
	return false
}

// IsRoleAllowed verifica si el rol del usuario satisface las restricciones del microservicio
func (s *ServiceConfig) IsRoleAllowed(role string) bool {
	if len(s.RequiredRoles) == 0 {
		return true // Sin restricción explícita de rol, rol válido permitido
	}
	for _, r := range s.RequiredRoles {
		if strings.EqualFold(r, role) {
			return true
		}
	}
	return false
}

// Config contiene todas las variables de configuración de la aplicación
type Config struct {
	Port                     string
	SecretKey                string
	GatewaySecret            string
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

	// Timeouts del servidor HTTP
	HTTPReadTimeout  time.Duration
	HTTPWriteTimeout time.Duration
	HTTPIdleTimeout  time.Duration

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

// AllowedOrigins parsea la variable ORIGIN (formato JSON Array o cadena simple) normalizando espacios y slashes finales
func (c *Config) AllowedOrigins() []string {
	raw := strings.TrimSpace(c.Origin)
	if raw == "" {
		return []string{"*"}
	}

	var candidates []string
	if strings.HasPrefix(raw, "[") {
		if err := json.Unmarshal([]byte(raw), &candidates); err != nil {
			candidates = nil
		}
	}
	if len(candidates) == 0 {
		candidates = strings.Split(raw, ",")
	}

	origins := make([]string, 0, len(candidates))
	for _, part := range candidates {
		cleaned := strings.TrimRight(strings.TrimSpace(part), "/")
		if cleaned == "" {
			continue
		}
		if cleaned == "*" {
			return []string{"*"}
		}
		origins = append(origins, cleaned)
	}

	if len(origins) == 0 {
		return []string{"*"}
	}
	return origins
}

// Load lee las variables de entorno desde .env (si existe) y las variables del sistema
func Load() (*Config, error) {
	// Intentar cargar .env desde el directorio de trabajo o directorio raíz
	_ = godotenv.Load()
	_ = godotenv.Load("../.env")

	secretKey := getEnv("SECRET_KEY", "")
	gatewaySecret := getEnv("GATEWAY_SECRET", "")
	if gatewaySecret == "" {
		gatewaySecret = secretKey
	}

	cfg := &Config{
		Port:                     getEnv("PORT", "8000"),
		SecretKey:                secretKey,
		GatewaySecret:            gatewaySecret,
		Algorithm:                getEnv("ALGORITHM", "HS256"),
		AccessTokenExpireMinutes: getEnvInt("ACCESS_TOKEN_EXPIRE_MINUTES", 60),

		DBUser:     getEnv("DB_USER", "postgres"),
		DBPassword: getEnv("DB_PASSWORD", "postgres"),
		DBHost:     getEnv("DB_HOST", "localhost"),
		DBPort:     getEnvInt("DB_PORT", 5432),
		DBName:     getEnv("DB_NAME", "gateway_auth"),

		EmailAdmin:    getEnv("EMAIL_ADMIN", "admin@admin.com"),
		PasswordAdmin: getEnv("PASSWORD_ADMIN", "Admin1234"),

		Origin: getEnv("ORIGIN", ""),

		HTTPReadTimeout:  time.Duration(getEnvInt("HTTP_READ_TIMEOUT", 15)) * time.Second,
		HTTPWriteTimeout: time.Duration(getEnvInt("HTTP_WRITE_TIMEOUT", 60)) * time.Second,
		HTTPIdleTimeout:  time.Duration(getEnvInt("HTTP_IDLE_TIMEOUT", 60)) * time.Second,

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
