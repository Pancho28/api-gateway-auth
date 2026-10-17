package proxy

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sort"
	"strings"
	"time"

	"api-gateway/internal/config"
	"api-gateway/internal/models"
	"api-gateway/internal/redis"
)

// Máximo tamaño de respuesta a almacenar en caché de Redis (1 MB) para prevenir OOM (Solución Vulnerabilidad 1)
const MaxCacheableBodySize = 1024 * 1024

// Cabeceras sensibles que se eliminan para prevenir Header Spoofing
var blockedHeaders = map[string]bool{
	"host":                true,
	"content-length":      true,
	"connection":          true,
	"keep-alive":          true,
	"proxy-authenticate":  true,
	"proxy-authorization": true,
	"te":                  true,
	"trailer":             true,
	"trailers":            true,
	"transfer-encoding":   true,
	"upgrade":             true,
	"x-internal-api-key":  true,
	"x-user-id":           true,
	"x-user-email":        true,
	"x-user-role":         true,
	"x-client-id":         true,
	"x-scopes":            true,
	"x-audience":          true,
}

// CachedResponse estructura para almacenar respuestas de peticiones GET en Redis
type CachedResponse struct {
	StatusCode int               `json:"status_code"`
	Headers    map[string]string `json:"headers"`
	Body       string            `json:"body"`
}

type Service struct {
	cfg   *config.Config
	redis *redis.Client
}

func NewService(cfg *config.Config, rdb *redis.Client) *Service {
	return &Service{
		cfg:   cfg,
		redis: rdb,
	}
}

// ForwardRequest procesa y redirige una petición HTTP hacia el microservicio correspondiente
func (s *Service) ForwardRequest(w http.ResponseWriter, r *http.Request, rawPath string, currentUser *models.User) {
	// 1. Sanitización de ruta contra ataques de Path Traversal
	prefix, subPath, err := s.sanitizePath(rawPath)
	if err != nil {
		http.Error(w, `{"detail":"Ruta de acceso inválida o no permitida [E-04]"}`, http.StatusBadRequest)
		return
	}

	serviceConfig, ok := s.cfg.Microservices[prefix]
	if !ok || serviceConfig.URL == "" {
		http.Error(w, `{"detail":"Recurso o servicio no encontrado [E-06]"}`, http.StatusNotFound)
		return
	}

	// 2. Particionamiento y aislamiento de clientes (Zero Trust / Deny by default - Opción B)
	clientID := ""
	role := ""
	if currentUser != nil {
		clientID = currentUser.ClientID
		role = currentUser.Role
	}

	if len(serviceConfig.AllowedClients) == 0 {
		http.Error(w, `{"detail":"Acceso al recurso denegado [E-03]"}`, http.StatusForbidden)
		return
	}

	if !serviceConfig.IsClientAllowed(clientID) {
		http.Error(w, `{"detail":"Acceso al recurso denegado [E-03]"}`, http.StatusForbidden)
		return
	}

	if len(serviceConfig.RequiredRoles) > 0 && !serviceConfig.IsRoleAllowed(role) {
		http.Error(w, `{"detail":"Acceso al recurso denegado [E-03]"}`, http.StatusForbidden)
		return
	}

	targetURL, err := url.Parse(serviceConfig.URL)
	if err != nil {
		http.Error(w, `{"detail":"Error interno del servidor [E-99]"}`, http.StatusInternalServerError)
		return
	}

	isGet := r.Method == http.MethodGet

	// 3. Interceptación de Caché en Redis para peticiones GET particionadas por cliente (ADR-002)
	var cacheKey string
	if isGet {
		cacheKey = s.generateCacheKey(prefix, subPath, clientID, r.URL.Query())
		var cached CachedResponse
		if s.redis.GetJSON(r.Context(), cacheKey, &cached) {
			for k, v := range cached.Headers {
				if strings.ToLower(k) != "content-length" {
					w.Header().Set(k, v)
				}
			}
			w.Header().Set("X-Cache", "HIT")
			w.WriteHeader(cached.StatusCode)
			_, _ = w.Write([]byte(cached.Body))
			return
		}
	}

	// 4. Configurar ReverseProxy con httputil
	proxy := &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			req.URL.Scheme = targetURL.Scheme
			req.URL.Host = targetURL.Host

			// Construir la ruta de destino sin duplicar slashes
			targetBasePath := strings.TrimRight(targetURL.Path, "/")
			if subPath != "" {
				req.URL.Path = targetBasePath + "/" + subPath
			} else {
				req.URL.Path = targetBasePath
			}
			req.URL.RawQuery = r.URL.RawQuery

			// Purgar cabeceras sensibles preveniendo spoofing
			for k := range req.Header {
				if blockedHeaders[strings.ToLower(k)] {
					req.Header.Del(k)
				}
			}

			// Inyectar API Key interna si está configurada
			if serviceConfig.APIKey != "" {
				req.Header.Set("X-Internal-Api-Key", serviceConfig.APIKey)
			}

			// Inyectar cabeceras seguras de identidad del usuario y cliente aguas abajo
			if currentUser != nil {
				req.Header.Set("X-User-ID", currentUser.ID)
				req.Header.Set("X-User-Email", currentUser.Email)
				req.Header.Set("X-User-Role", currentUser.Role)
				if currentUser.ClientID != "" {
					req.Header.Set("X-Client-ID", currentUser.ClientID)
				}
				if len(currentUser.Scopes) > 0 {
					req.Header.Set("X-Scopes", strings.Join(currentUser.Scopes, " "))
				}
			}
		},
		ErrorHandler: func(rw http.ResponseWriter, req *http.Request, proxyErr error) {
			log.Printf("[PROXY ERROR] Fallo al comunicar con microservicio '%s': %v", prefix, proxyErr)
			rw.Header().Set("Content-Type", "application/json")
			rw.WriteHeader(http.StatusBadGateway)
			_, _ = rw.Write([]byte(`{"detail":"Error de comunicación con el servicio de destino [E-05]"}`))
		},
	}

	// 4. Si es petición GET, interceptar respuesta para cachear con límite seguro de memoria
	if isGet {
		proxy.ModifyResponse = func(resp *http.Response) error {
			resp.Header.Set("X-Cache", "MISS")
			if resp.StatusCode == http.StatusOK && cacheKey != "" {
				// Leer con límite de seguridad (máximo 1 MB) para prevenir OOM
				limitedReader := io.LimitReader(resp.Body, MaxCacheableBodySize+1)
				bodyBytes, err := io.ReadAll(limitedReader)
				if err == nil {
					// Restaurar el body original para que continúe el streaming al cliente
					resp.Body = io.NopCloser(bytes.NewReader(bodyBytes))

					// Si no superó el límite máximo, guardar en Redis de forma asíncrona
					if len(bodyBytes) <= MaxCacheableBodySize {
						headersMap := make(map[string]string)
						for k, v := range resp.Header {
							headersMap[k] = strings.Join(v, ", ")
						}
						cachedObj := CachedResponse{
							StatusCode: resp.StatusCode,
							Headers:    headersMap,
							Body:       string(bodyBytes),
						}
						go func() {
							ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
							defer cancel()
							s.redis.SetJSON(ctx, cacheKey, cachedObj, s.cfg.RedisCacheTTL)
						}()
					}
				}
			}
			return nil
		}
	}

	// 5. Ejecutar reenvío
	proxy.ServeHTTP(w, r)
}

func (s *Service) sanitizePath(path string) (prefix, subPath string, err error) {
	unquoted, err := url.PathUnescape(path)
	if err != nil {
		return "", "", fmt.Errorf("ruta de acceso inválida")
	}

	if strings.Contains(unquoted, "\x00") {
		return "", "", fmt.Errorf("ruta de acceso inválida o no permitida")
	}

	clean := strings.Trim(unquoted, "/")
	if clean == "" {
		return "", "", fmt.Errorf("ruta de microservicio no especificada")
	}

	segments := strings.Split(clean, "/")
	var validSegments []string
	for _, seg := range segments {
		if seg == "" {
			continue
		}
		if seg == "." || seg == ".." || strings.HasPrefix(seg, "..") || strings.HasSuffix(seg, "..") {
			return "", "", fmt.Errorf("ruta de acceso inválida: intento de path traversal detectado")
		}
		validSegments = append(validSegments, seg)
	}

	if len(validSegments) == 0 {
		return "", "", fmt.Errorf("ruta de microservicio no especificada")
	}

	prefix = validSegments[0]
	if len(validSegments) > 1 {
		subPath = strings.Join(validSegments[1:], "/")
	}
	return prefix, subPath, nil
}

func (s *Service) generateCacheKey(prefix, subPath, clientID string, query url.Values) string {
	keys := make([]string, 0, len(query))
	for k := range query {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var queryParts []string
	for _, k := range keys {
		vals := query[k]
		sort.Strings(vals)
		for _, v := range vals {
			queryParts = append(queryParts, fmt.Sprintf("%s=%s", k, v))
		}
	}
	queryStr := strings.Join(queryParts, "&")
	rawToHash := fmt.Sprintf("%s?%s", subPath, queryStr)

	h := sha256.New()
	h.Write([]byte(rawToHash))
	hashDigest := hex.EncodeToString(h.Sum(nil))

	if clientID != "" {
		return fmt.Sprintf("proxy_cache:%s:%s:%s", prefix, clientID, hashDigest)
	}
	return fmt.Sprintf("proxy_cache:%s:%s", prefix, hashDigest)
}
