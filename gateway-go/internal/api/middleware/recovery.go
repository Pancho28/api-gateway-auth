package middleware

import (
	"fmt"
	"log"
	"net/http"

	"github.com/google/uuid"
)

// Recoverer captura panics no controlados, genera un ErrorID único y sanitiza los logs (Error 500)
func Recoverer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rvr := recover(); rvr != nil {
				errorID := uuid.New().String()[:12]
				log.Printf("[CRITICAL ERROR] Error interno no controlado [ErrorID: %s] en %s %s: %v",
					errorID, r.Method, r.URL.Path, rvr)

				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("X-Error-ID", errorID)
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte(fmt.Sprintf(`{"detail":"Error interno del servidor"}`)))
			}
		}()
		next.ServeHTTP(w, r)
	})
}
