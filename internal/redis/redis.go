package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"api-gateway/internal/config"
	"github.com/redis/go-redis/v9"
)

// Client envuelve el cliente oficial de Redis con soporte para degradación elegante
type Client struct {
	rdb *redis.Client
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
	if c.rdb == nil {
		return false
	}

	payload, err := json.Marshal(value)
	if err != nil {
		log.Printf("[REDIS] Error al serializar JSON para clave '%s': %v", key, err)
		return false
	}

	if err := c.rdb.Set(ctx, key, payload, ttl).Err(); err != nil {
		log.Printf("[REDIS] Fallo recuperable al guardar clave '%s': %v", key, err)
		return false
	}

	return true
}

// Delete elimina una clave de Redis de forma segura
func (c *Client) Delete(ctx context.Context, key string) bool {
	if c.rdb == nil {
		return false
	}

	if err := c.rdb.Del(ctx, key).Err(); err != nil {
		log.Printf("[REDIS] Fallo recuperable al eliminar clave '%s': %v", key, err)
		return false
	}

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
