package handlers

import (
	"encoding/json"
	"net/http"
	"strings"

	"api-gateway/internal/api/middleware"
	"api-gateway/internal/proxy"
)

type GatewayHandler struct {
	proxyService *proxy.Service
}

func NewGatewayHandler(proxyService *proxy.Service) *GatewayHandler {
	return &GatewayHandler{proxyService: proxyService}
}

// HealthCheck responde al estado de salud del API Gateway (/health)
func (h *GatewayHandler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"status":          "healthy",
		"gateway_version": "1.0.0",
		"system":          "API Gateway Go",
	})
}

// ReverseProxy captura todas las rutas restantes (catch-all) y redirige al microservicio
func (h *GatewayHandler) ReverseProxy(w http.ResponseWriter, r *http.Request) {
	// Obtener la ruta relativa después del slash inicial
	rawPath := strings.TrimPrefix(r.URL.Path, "/")
	currentUser := middleware.GetUserFromContext(r.Context())

	h.proxyService.ForwardRequest(w, r, rawPath, currentUser)
}
