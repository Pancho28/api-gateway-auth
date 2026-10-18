package models

import (
	"errors"
	"regexp"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

var (
	emailRegex    = regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
	hasLetterRegex = regexp.MustCompile(`[a-zA-Z]`)
	hasDigitRegex  = regexp.MustCompile(`[0-9]`)
	clientRegex   = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)
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

// Roles válidos reconocidos en el ecosistema
var ValidRoles = map[string]bool{
	"USER":       true,
	"ADMIN":      true,
	"CUSTOMER":   true,
	"STAFF":      true,
	"SUPERADMIN": true,
}

// IsValidRole valida si una cadena corresponde a un rol admitido
func IsValidRole(role string) bool {
	return ValidRoles[strings.ToUpper(strings.TrimSpace(role))]
}

// UserRegisterRequest representa el cuerpo de la petición pública para /auth/register
type UserRegisterRequest struct {
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

	// Asignar rol por defecto para registro público si no viene especificado
	if strings.TrimSpace(r.Role) == "" {
		r.Role = "CUSTOMER"
	} else {
		r.Role = strings.ToUpper(strings.TrimSpace(r.Role))
		// Impedir auto-asignación de roles privilegiados en registro perimetral abierto
		if r.Role == "ADMIN" || r.Role == "SUPERADMIN" || r.Role == "STAFF" {
			return errors.New("no está permitido registrarse con roles administrativos o de staff")
		}
		if !IsValidRole(r.Role) {
			return errors.New("el rol especificado no es válido")
		}
	}
	return nil
}

// AdminUserCreateRequest representa la petición de creación de usuario por un ADMIN
type AdminUserCreateRequest struct {
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

	r.Role = strings.ToUpper(strings.TrimSpace(r.Role))
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
