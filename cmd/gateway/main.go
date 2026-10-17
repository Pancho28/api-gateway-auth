package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"api-gateway/internal/api"
	"api-gateway/internal/config"
	"api-gateway/internal/database"
	"api-gateway/internal/redis"
)

func main() {
	log.Println("==================================================")
	log.Println("🚀 Iniciando API Gateway & Centralized Auth (Go)")
	log.Println("==================================================")

	// 1. Cargar Configuración
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Error crítico al cargar configuración: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 2. Conectar a PostgreSQL (pgxpool)
	db, err := database.NewPostgresPool(ctx, cfg)
	if err != nil {
		log.Fatalf("Error crítico al conectar a base de datos: %v", err)
	}
	defer db.Close()

	// Inicializar tablas e índices
	log.Println("Verificando e inicializando tablas en PostgreSQL...")
	if err := db.InitSchema(ctx); err != nil {
		log.Printf("Advertencia en inicialización de esquema: %v", err)
	}

	// 3. Inicializar administrador por defecto (Seeding)
	if err := database.InitializeAdmin(ctx, db, cfg); err != nil {
		log.Printf("Advertencia en seeding de administrador: %v", err)
	}

	// 4. Conectar a Redis
	rdb := redis.NewClient(cfg)
	defer func() {
		_ = rdb.Close()
	}()

	// 5. Configurar Enrutamiento y Middlewares
	router := api.SetupRouter(cfg, db, rdb)

	server := &http.Server{
		Addr:         fmt.Sprintf(":%s", cfg.Port),
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Canal para captura de señales de apagado del sistema operativo (Graceful Shutdown)
	stopChan := make(chan os.Signal, 1)
	signal.Notify(stopChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		log.Printf("API Gateway escuchando en el puerto :%s", cfg.Port)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("Fallo en servidor HTTP: %v", err)
		}
	}()

	// Bloquear hasta recibir señal de terminación
	<-stopChan
	log.Println("\nApagando API Gateway, liberando conexiones y recursos...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("Error al cerrar servidor HTTP: %v", err)
	}

	log.Println("Recursos liberados. Apagado exitoso.")
}
