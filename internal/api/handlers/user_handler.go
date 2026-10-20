package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"api-gateway/internal/models"
	"api-gateway/internal/user"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type UserHandler struct {
	userService *user.Service
}

func NewUserHandler(userService *user.Service) *UserHandler {
	return &UserHandler{userService: userService}
}

// CreateUser permite a un administrador crear nuevos usuarios en el sistema (/users)
func (h *UserHandler) CreateUser(w http.ResponseWriter, r *http.Request) {
	var req models.AdminUserCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnprocessableEntity)
		_ = json.NewEncoder(w).Encode(map[string]string{"detail": "Formato de solicitud o cuerpo JSON no válido [E-10]"})
		return
	}

	if err := req.Validate(); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"detail": "Parámetros de solicitud no válidos [E-11]"})
		return
	}

	res, err := h.userService.CreateUserByAdmin(r.Context(), &req)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		if errors.Is(err, user.ErrEmailAlreadyExists) {
			w.WriteHeader(http.StatusConflict)
			_ = json.NewEncoder(w).Encode(map[string]string{"detail": "Conflicto con el recurso solicitado [E-12]"})
			return
		}
		if errors.Is(err, user.ErrAdminRoleForbidden) {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{"detail": "Parámetros de solicitud no válidos [E-11]"})
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"detail": "Error interno del servidor [E-99]"})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(res)
}

// CreateUserM2M procesa la creación de credenciales desde backends autorizados (/auth/admin/users)
func (h *UserHandler) CreateUserM2M(w http.ResponseWriter, r *http.Request) {
	var req models.M2MUserCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnprocessableEntity)
		_ = json.NewEncoder(w).Encode(map[string]string{"detail": "Formato de solicitud o cuerpo JSON no válido [E-10]"})
		return
	}

	if err := req.Validate(); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"detail": "Parámetros de solicitud no válidos [E-11]"})
		return
	}

	res, err := h.userService.CreateUserM2M(r.Context(), &req)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		if errors.Is(err, user.ErrEmailAlreadyExists) {
			w.WriteHeader(http.StatusConflict)
			_ = json.NewEncoder(w).Encode(map[string]string{"detail": "Conflicto con el recurso solicitado [E-12]"})
			return
		}
		if errors.Is(err, user.ErrAdminRoleForbidden) {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{"detail": "Parámetros de solicitud no válidos [E-11]"})
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"detail": "Error interno del servidor [E-99]"})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(res)
}

// DeleteUserM2M elimina un usuario para operaciones de compensación/rollback (/auth/admin/users/{id})
func (h *UserHandler) DeleteUserM2M(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(chi.URLParam(r, "id"))
	if id == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"detail": "Parámetros de solicitud no válidos [E-11]"})
		return
	}

	if _, err := uuid.Parse(id); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"detail": "Parámetros de solicitud no válidos [E-11]"})
		return
	}

	if err := h.userService.DeleteUserM2M(r.Context(), id); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"detail": "Error interno del servidor [E-99]"})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{"detail": "Credencial eliminada exitosamente"})
}
