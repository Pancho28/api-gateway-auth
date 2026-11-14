package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"api-gateway/internal/config"
	"github.com/stretchr/testify/assert"
)

func TestSetupCORSMultiOrigin(t *testing.T) {
	cfg := &config.Config{
		Origin: `["https://front1.com", "https://front2.com/", ""]`,
	}

	handler := SetupCORS(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// 1. Primer origen permitido (preflight OPTIONS)
	req1 := httptest.NewRequest(http.MethodOptions, "/auth/login", nil)
	req1.Header.Set("Origin", "https://front1.com")
	req1.Header.Set("Access-Control-Request-Method", "POST")
	rec1 := httptest.NewRecorder()
	handler.ServeHTTP(rec1, req1)

	assert.Equal(t, "https://front1.com", rec1.Header().Get("Access-Control-Allow-Origin"))
	assert.Equal(t, "true", rec1.Header().Get("Access-Control-Allow-Credentials"))

	// 2. Segundo origen permitido (sin trailing slash normalizado)
	req2 := httptest.NewRequest(http.MethodGet, "/health", nil)
	req2.Header.Set("Origin", "https://front2.com")
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)

	assert.Equal(t, http.StatusOK, rec2.Code)
	assert.Equal(t, "https://front2.com", rec2.Header().Get("Access-Control-Allow-Origin"))
	assert.Equal(t, "true", rec2.Header().Get("Access-Control-Allow-Credentials"))

	// 3. Origen no autorizado -> No debe incluir Access-Control-Allow-Origin
	reqBad := httptest.NewRequest(http.MethodGet, "/health", nil)
	reqBad.Header.Set("Origin", "https://intruso.com")
	recBad := httptest.NewRecorder()
	handler.ServeHTTP(recBad, reqBad)

	assert.Empty(t, recBad.Header().Get("Access-Control-Allow-Origin"))
}

func TestSetupCORSWildcardDoesNotSendCredentials(t *testing.T) {
	cfg := &config.Config{
		Origin: "",
	}

	handler := SetupCORS(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	req.Header.Set("Origin", "https://cualquiera.com")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, "*", rec.Header().Get("Access-Control-Allow-Origin"))
	assert.Empty(t, rec.Header().Get("Access-Control-Allow-Credentials"))
}
