package handlers

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"

	"api-gateway/internal/auth"
	"api-gateway/internal/models"
)

type AuthHandler struct {
	authService *auth.Service
}

func NewAuthHandler(authService *auth.Service) *AuthHandler {
	return &AuthHandler{authService: authService}
}

// Login procesa la petición de inicio de sesión (/auth/login)
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req models.UserLoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("[AUTH] [LOGIN_REJECTED] Formato JSON inválido | IP: %s", r.RemoteAddr)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnprocessableEntity)
		_ = json.NewEncoder(w).Encode(map[string]string{"detail": "Formato de solicitud o cuerpo JSON no válido [E-10]"})
		return
	}

	// Punto 2: Exigir estrictamente X-Client-Id (por header o por body) con mensaje opaco [E-01]
	if strings.TrimSpace(req.ClientID) == "" {
		req.ClientID = strings.TrimSpace(r.Header.Get("X-Client-Id"))
	}
	if req.ClientID == "" {
		log.Printf("[AUTH] [LOGIN_REJECTED] Intento de login sin Client-ID para usuario '%s' | IP: %s", req.Email, r.RemoteAddr)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"detail": "Cabeceras requeridas ausentes o no válidas [E-01]",
		})
		return
	}

	if err := req.Validate(); err != nil {
		log.Printf("[AUTH] [LOGIN_REJECTED] Parámetros inválidos para usuario '%s' | Cliente: '%s' | IP: %s", req.Email, req.ClientID, r.RemoteAddr)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"detail": "Parámetros de solicitud no válidos [E-11]"})
		return
	}

	res, err := h.authService.Login(r.Context(), &req)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		if errors.Is(err, auth.ErrInvalidCredentials) {
			log.Printf("[AUTH] [LOGIN_FAILED] Intento fallido para usuario '%s' | Cliente: '%s' | Motivo: Credenciales inválidas | IP: %s", req.Email, req.ClientID, r.RemoteAddr)
			w.Header().Set("WWW-Authenticate", "Bearer")
			w.WriteHeader(http.StatusUnauthorized)
			_ = json.NewEncoder(w).Encode(map[string]string{"detail": "No se pudieron validar las credenciales [E-08]"})
			return
		}
		log.Printf("[AUTH] [LOGIN_ERROR] Error interno al procesar login de '%s' | Cliente: '%s': %v | IP: %s", req.Email, req.ClientID, err, r.RemoteAddr)
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"detail": "Error interno del servidor [E-99]"})
		return
	}

	log.Printf("[AUTH] [LOGIN_SUCCESS] Inicio de sesión exitoso: usuario '%s' para cliente '%s' (Rol: %s) | IP: %s", res.User, res.ClientID, res.Role, r.RemoteAddr)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(res)
}

// Register procesa el registro perimetral público de comensales/usuarios (/auth/register)
func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req models.UserRegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnprocessableEntity)
		_ = json.NewEncoder(w).Encode(map[string]string{"detail": "Formato de solicitud o cuerpo JSON no válido [E-10]"})
		return
	}

	if strings.TrimSpace(req.ClientID) == "" {
		req.ClientID = strings.TrimSpace(r.Header.Get("X-Client-Id"))
	}
	if req.ClientID == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"detail": "Cabeceras requeridas ausentes o no válidas [E-01]",
		})
		return
	}

	if err := req.Validate(); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"detail": "Parámetros de solicitud no válidos [E-11]"})
		return
	}

	res, err := h.authService.Register(r.Context(), &req)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		if errors.Is(err, auth.ErrEmailAlreadyExists) {
			log.Printf("[AUTH] [REGISTER_CONFLICT] Intento de registro con correo ya existente: '%s' | Cliente: '%s' | IP: %s", req.Email, req.ClientID, r.RemoteAddr)
			w.WriteHeader(http.StatusConflict)
			_ = json.NewEncoder(w).Encode(map[string]string{"detail": "Conflicto con el recurso solicitado [E-12]"})
			return
		}
		log.Printf("[AUTH] [REGISTER_ERROR] Error interno al registrar usuario '%s': %v | IP: %s", req.Email, err, r.RemoteAddr)
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"detail": "Error interno del servidor [E-99]"})
		return
	}

	log.Printf("[AUTH] [REGISTER_SUCCESS] Usuario registrado exitosamente: '%s' para cliente '%s' (Rol: %s) | IP: %s", res.User, res.ClientID, res.Role, r.RemoteAddr)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(res)
}

