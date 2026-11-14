package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"api-gateway/internal/config"
	"github.com/redis/go-redis/v9"
)

// SessionKey construye la clave de sesión en Redis particionada por email y client_id
func SessionKey(email, clientID string) string {
	cleanClient := strings.TrimSpace(clientID)
	if cleanClient != "" {
		return fmt.Sprintf("session:%s:%s", email, cleanClient)
	}
	return fmt.Sprintf("session:%s", email)
}

// RevokedJWTKey construye la clave en Redis para la denylist de JWTs revocados por jti
func RevokedJWTKey(jti string) string {
	return fmt.Sprintf("revoked_jwt:%s", jti)
}

type memEntry struct {
	value     string
	expiresAt time.Time
}

// Client envuelve el cliente oficial de Redis con soporte para degradación elegante
type Client struct {
	rdb *redis.Client
	mu  sync.RWMutex
	mem map[string]memEntry
}

// NewInMemoryClient crea un cliente en memoria aislado para pruebas sin requerir servidor Redis
func NewInMemoryClient() *Client {
	return &Client{
		mem: make(map[string]memEntry),
	}
}

// NewClient inicializa el pool de conexiones asíncrono para Redis
func NewClient(cfg *config.Config) *Client {
	rdb := redis.NewClient(&redis.Options{
		Addr:         cfg.RedisAddr(),
		Password:     cfg.RedisPassword,
		DB:           cfg.RedisDB,
		PoolSize:     50,
		MinIdleConns: 5,
		DialTimeout:  2 * time.Second,
		ReadTimeout:  1 * time.Second,
		WriteTimeout: 1 * time.Second,
	})

	// Intento de ping inicial no bloqueante
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := rdb.Ping(ctx).Err(); err != nil {
		log.Printf("[REDIS] Advertencia: Conexión inicial a Redis no disponible: %v (conmutando a degradación elegante)", err)
	}

	return &Client{rdb: rdb}
}

// GetClient expone el cliente subyacente para componentes como el rate limiter
func (c *Client) GetClient() *redis.Client {
	return c.rdb
}

// GetJSON recupera un valor JSON deserializándolo en la estructura de destino.
// Implementa Graceful Degradation: si Redis falla o no tiene la clave, retorna false/nil.
func (c *Client) GetJSON(ctx context.Context, key string, dest interface{}) bool {
	if c.mem != nil {
		val, ok := c.getMem(key)
		if !ok {
			return false
		}
		if err := json.Unmarshal([]byte(val), dest); err != nil {
			log.Printf("[REDIS] Error al deserializar JSON para clave '%s': %v", key, err)
			return false
		}
		return true
	}

	if c.rdb == nil {
		return false
	}

	val, err := c.rdb.Get(ctx, key).Result()
	if err != nil {
		if err != redis.Nil {
			log.Printf("[REDIS] Fallo recuperable al obtener clave '%s': %v", key, err)
		}
		return false
	}

	if err := json.Unmarshal([]byte(val), dest); err != nil {
		log.Printf("[REDIS] Error al deserializar JSON para clave '%s': %v", key, err)
		return false
	}

	return true
}

// SetJSON serializa y guarda un valor en Redis con TTL.
// Retorna true si fue exitoso o false en caso de degradación por error.
func (c *Client) SetJSON(ctx context.Context, key string, value interface{}, ttl time.Duration) bool {
	if c.mem == nil && c.rdb == nil {
		return false
	}

	payload, err := json.Marshal(value)
	if err != nil {
		log.Printf("[REDIS] Error al serializar JSON para clave '%s': %v", key, err)
		return false
	}

	if c.mem != nil {
		return c.setMem(key, string(payload), ttl)
	}

	if err := c.rdb.Set(ctx, key, payload, ttl).Err(); err != nil {
		log.Printf("[REDIS] Fallo recuperable al guardar clave '%s': %v", key, err)
		return false
	}

	return true
}

// Delete elimina una clave de Redis de forma segura
func (c *Client) Delete(ctx context.Context, key string) bool {
	if c.mem != nil {
		c.mu.Lock()
		delete(c.mem, key)
		c.mu.Unlock()
		return true
	}

	if c.rdb == nil {
		return false
	}

	if err := c.rdb.Del(ctx, key).Err(); err != nil {
		log.Printf("[REDIS] Fallo recuperable al eliminar clave '%s': %v", key, err)
		return false
	}

	return true
}

// SetString almacena un valor de texto plano en Redis con TTL
func (c *Client) SetString(ctx context.Context, key, value string, ttl time.Duration) bool {
	if ttl <= 0 {
		return false
	}

	if c.mem != nil {
		return c.setMem(key, value, ttl)
	}

	if c.rdb == nil {
		return false
	}

	if err := c.rdb.Set(ctx, key, value, ttl).Err(); err != nil {
		log.Printf("[REDIS] Fallo recuperable al guardar clave '%s': %v", key, err)
		return false
	}

	return true
}

// Exists verifica de forma segura si una clave existe en Redis
func (c *Client) Exists(ctx context.Context, key string) bool {
	if c.mem != nil {
		_, ok := c.getMem(key)
		return ok
	}

	if c.rdb == nil {
		return false
	}

	count, err := c.rdb.Exists(ctx, key).Result()
	if err != nil {
		log.Printf("[REDIS] Fallo recuperable al verificar existencia de clave '%s': %v", key, err)
		return false
	}

	return count > 0
}

func (c *Client) getMem(key string) (string, bool) {
	c.mu.RLock()
	entry, ok := c.mem[key]
	c.mu.RUnlock()
	if !ok {
		return "", false
	}
	if !entry.expiresAt.IsZero() && time.Now().After(entry.expiresAt) {
		c.mu.Lock()
		delete(c.mem, key)
		c.mu.Unlock()
		return "", false
	}
	return entry.value, true
}

func (c *Client) setMem(key, value string, ttl time.Duration) bool {
	var exp time.Time
	if ttl > 0 {
		exp = time.Now().Add(ttl)
	}
	c.mu.Lock()
	c.mem[key] = memEntry{value: value, expiresAt: exp}
	c.mu.Unlock()
	return true
}

// Close cierra el pool de conexiones de Redis
func (c *Client) Close() error {
	if c.rdb != nil {
		return c.rdb.Close()
	}
	return nil
}

// Ping verifica el estado de salud de Redis
func (c *Client) Ping(ctx context.Context) error {
	if c.rdb == nil {
		return fmt.Errorf("cliente de Redis no inicializado")
	}
	return c.rdb.Ping(ctx).Err()
}
