package handlers

import (
	"encoding/json"
	"errors"
	"net/http"

	"api-gateway/internal/models"
	"api-gateway/internal/user"
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
		_ = json.NewEncoder(w).Encode(map[string]string{"detail": "Cuerpo de petición inválido"})
		return
	}

	if err := req.Validate(); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"detail": err.Error()})
		return
	}

	res, err := h.userService.CreateUserByAdmin(r.Context(), &req)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		if errors.Is(err, user.ErrEmailAlreadyExists) {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{"detail": err.Error()})
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"detail": "Error interno del servidor al crear el usuario."})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(res)
}
