package models

import (
	"errors"
	"regexp"
	"strings"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

var (
	emailRegex    = regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
	hasLetterRegex = regexp.MustCompile(`[a-zA-Z]`)
	hasDigitRegex  = regexp.MustCompile(`[0-9]`)
	clientRegex   = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)
)

// Constantes de roles a nivel de sistema del Gateway
const (
	RoleAdmin = "admin"
	RoleUser  = "user"
)

// User representa el modelo de entidad en la base de datos y en el contexto de autenticación
type User struct {
	ID             string   `json:"id"`
	Email          string   `json:"email"`
	HashedPassword string   `json:"-"`
	Role           string   `json:"role"`
	ClientID       string   `json:"client_id,omitempty"`
	Scopes         []string `json:"scopes,omitempty"`
}

// UserLoginRequest representa el cuerpo de la petición para /auth/login
type UserLoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	ClientID string `json:"client_id,omitempty"`
}

func (r *UserLoginRequest) Validate() error {
	r.Email = strings.TrimSpace(r.Email)
	if r.Email == "" || len(r.Email) > 50 {
		return errors.New("el correo electrónico es inválido o excede los 50 caracteres")
	}
	if !emailRegex.MatchString(r.Email) {
		return errors.New("formato de correo electrónico inválido")
	}
	if r.Password == "" || len(r.Password) > 40 {
		return errors.New("la contraseña es inválida o excede los 40 caracteres")
	}
	r.ClientID = strings.TrimSpace(r.ClientID)
	if r.ClientID != "" {
		if len(r.ClientID) > 50 || !clientRegex.MatchString(r.ClientID) {
			return errors.New("el identificador de cliente es inválido (máx 50 caracteres alfanuméricos, guiones o guiones bajos)")
		}
	}
	return nil
}

// TokenResponse representa la respuesta entregada tras un login exitoso
type TokenResponse struct {
	User     string `json:"user"`
	Role     string `json:"role"`
	Token    string `json:"token"`
	ClientID string `json:"client_id,omitempty"`
}

// Roles válidos reconocidos en el sistema
var ValidRoles = map[string]bool{
	RoleAdmin: true,
	RoleUser:  true,
}

// IsValidRole valida si una cadena corresponde a un rol admitido (case-insensitive)
func IsValidRole(role string) bool {
	return ValidRoles[strings.ToLower(strings.TrimSpace(role))]
}

// UserRegisterRequest representa el cuerpo de la petición pública para /auth/register
type UserRegisterRequest struct {
	ID       string `json:"id,omitempty"`
	Email    string `json:"email"`
	Password string `json:"password"`
	ClientID string `json:"client_id,omitempty"`
	Role     string `json:"role,omitempty"`
}

func (r *UserRegisterRequest) Validate() error {
	r.Email = strings.TrimSpace(r.Email)
	if r.Email == "" || len(r.Email) > 50 {
		return errors.New("el correo electrónico es obligatorio y no debe exceder 50 caracteres")
	}
	if !emailRegex.MatchString(r.Email) {
		return errors.New("formato de correo electrónico inválido")
	}

	if len(r.Password) < 8 || len(r.Password) > 40 {
		return errors.New("la contraseña debe tener entre 8 y 40 caracteres")
	}
	if !hasLetterRegex.MatchString(r.Password) || !hasDigitRegex.MatchString(r.Password) {
		return errors.New("la contraseña debe contener al menos una letra y un número")
	}

	r.ClientID = strings.TrimSpace(r.ClientID)
	if r.ClientID != "" {
		if len(r.ClientID) > 50 || !clientRegex.MatchString(r.ClientID) {
			return errors.New("el identificador de cliente es inválido")
		}
	}

	r.ID = strings.TrimSpace(r.ID)
	if r.ID != "" {
		if _, err := uuid.Parse(r.ID); err != nil {
			return errors.New("el identificador de usuario 'id' debe ser un UUIDv4 válido")
		}
	}

	// Asignar rol por defecto para registro público si no viene especificado
	if strings.TrimSpace(r.Role) == "" {
		r.Role = RoleUser
	} else {
		r.Role = strings.ToLower(strings.TrimSpace(r.Role))
		// Impedir auto-asignación de rol administrativo en registro perimetral abierto
		if r.Role == RoleAdmin {
			return errors.New("no está permitido registrarse con rol administrativo")
		}
		if !IsValidRole(r.Role) {
			return errors.New("el rol especificado no es válido")
		}
	}
	return nil
}

// M2MUserCreateRequest representa la petición de creación de usuario desde un backend M2M (/auth/admin/users)
type M2MUserCreateRequest struct {
	ID       string                 `json:"id,omitempty"`
	Email    string                 `json:"email"`
	Password string                 `json:"password"`
	ClientID string                 `json:"client_id"`
	Role     string                 `json:"role,omitempty"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

func (r *M2MUserCreateRequest) Validate() error {
	r.Email = strings.TrimSpace(r.Email)
	if r.Email == "" || len(r.Email) > 50 {
		return errors.New("el correo electrónico es obligatorio y no debe exceder 50 caracteres")
	}
	if !emailRegex.MatchString(r.Email) {
		return errors.New("formato de correo electrónico inválido")
	}

	if len(r.Password) < 8 || len(r.Password) > 40 {
		return errors.New("la contraseña debe tener entre 8 y 40 caracteres")
	}
	if !hasLetterRegex.MatchString(r.Password) || !hasDigitRegex.MatchString(r.Password) {
		return errors.New("la contraseña debe contener al menos una letra y un número")
	}

	r.ClientID = strings.TrimSpace(r.ClientID)
	if r.ClientID == "" || len(r.ClientID) > 50 || !clientRegex.MatchString(r.ClientID) {
		return errors.New("el identificador de cliente es obligatorio e inválido")
	}

	r.ID = strings.TrimSpace(r.ID)
	if r.ID != "" {
		if _, err := uuid.Parse(r.ID); err != nil {
			return errors.New("el identificador de usuario 'id' debe ser un UUIDv4 válido")
		}
	}

	if strings.TrimSpace(r.Role) == "" {
		r.Role = RoleUser
	} else {
		r.Role = strings.ToLower(strings.TrimSpace(r.Role))
		if r.Role == RoleAdmin {
			return errors.New("está estrictamente prohibido crear usuarios con rol 'admin' vía API")
		}
		if !IsValidRole(r.Role) {
			return errors.New("el rol especificado no es válido")
		}
	}
	return nil
}

// M2MUserResponse representa la respuesta devuelta tras la creación de usuario M2M
type M2MUserResponse struct {
	ID       string `json:"id"`
	Email    string `json:"email"`
	Role     string `json:"role"`
	ClientID string `json:"client_id,omitempty"`
}

// AdminUserCreateRequest representa la petición de creación de usuario por un ADMIN (/users)
type AdminUserCreateRequest struct {
	ID       string `json:"id,omitempty"`
	Email    string `json:"email"`
	Password string `json:"password"`
	Role     string `json:"role"`
}

func (r *AdminUserCreateRequest) Validate() error {
	r.Email = strings.TrimSpace(r.Email)
	if r.Email == "" || len(r.Email) > 50 {
		return errors.New("el correo electrónico es obligatorio y no debe exceder 50 caracteres")
	}
	if !emailRegex.MatchString(r.Email) {
		return errors.New("formato de correo electrónico inválido")
	}

	if len(r.Password) < 8 || len(r.Password) > 40 {
		return errors.New("la contraseña debe tener entre 8 y 40 caracteres")
	}
	if !hasLetterRegex.MatchString(r.Password) || !hasDigitRegex.MatchString(r.Password) {
		return errors.New("la contraseña debe contener al menos una letra y un número")
	}

	r.ID = strings.TrimSpace(r.ID)
	if r.ID != "" {
		if _, err := uuid.Parse(r.ID); err != nil {
			return errors.New("el identificador de usuario 'id' debe ser un UUIDv4 válido")
		}
	}

	r.Role = strings.ToLower(strings.TrimSpace(r.Role))
	if r.Role == RoleAdmin {
		return errors.New("está estrictamente prohibido crear usuarios con rol 'admin' vía API")
	}
	if !IsValidRole(r.Role) {
		return errors.New("el rol especificado no es válido")
	}
	return nil
}

// UserResponse representa la respuesta devuelta al crear un nuevo usuario
type UserResponse struct {
	ID    string `json:"id"`
	Email string `json:"email"`
	Role  string `json:"role"`
}

// JWTClaims representa la información decodificada del token con soporte de audiencia y client_id
type JWTClaims struct {
	jwt.RegisteredClaims
	Role     string   `json:"role"`
	ClientID string   `json:"client_id"`
	Scopes   []string `json:"scopes,omitempty"`
}

// SessionData representa los datos de sesión almacenados en Redis
type SessionData struct {
	ID       string   `json:"id"`
	Email    string   `json:"email"`
	Role     string   `json:"role"`
	ClientID string   `json:"client_id"`
	Scopes   []string `json:"scopes,omitempty"`
}
