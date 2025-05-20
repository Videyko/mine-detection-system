package main

import (
	"context"
	"database/sql"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	_ "github.com/lib/pq"

	"mine-detection-system/internal/application"
	"mine-detection-system/internal/infrastructure/repositories"
	"mine-detection-system/internal/ports/api"
	"mine-detection-system/internal/ports/ws"
)

func main() {
	// Парсинг командного рядка
	var (
		addr  = flag.String("addr", ":8080", "HTTP server address")
		dbURL = flag.String("db", "postgres://postgres:postgres@localhost/mine_detection?sslmode=disable", "Database URL")
	)
	flag.Parse()

	// Підключення до бази даних
	db, err := sql.Open("postgres", *dbURL)
	if err != nil {
		log.Fatalf("Error connecting to database: %v", err)
	}
	defer db.Close()

	// Створення репозиторіїв
	deviceRepo := repositories.NewPostgresDeviceRepository(db)
	// Тут створення інших репозиторіїв...

	// Створення сервісів
	deviceService := application.NewDeviceService(deviceRepo)
	// Тут створення інших сервісів...

	// Створення HTTP-обробників
	deviceHandler := api.NewDeviceHandler(deviceService)
	// Тут створення інших обробників...

	// Налаштування WebSocket обробника для сенсорів
	sensorWSHandler := ws.NewSensorHandler(nil, deviceService) // Замість nil має бути sensorService

	// Налаштування маршрутизатора
	r := chi.NewRouter()

	// Middleware
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(60 * time.Second))

	// CORS
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"}, // В продакшені має бути обмеження
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300, // Maximum value not caught in preflight cache
	}))

	// API версіонування
	r.Route("/api", func(r chi.Router) {
		r.Route("/v1", func(r chi.Router) {
			// Реєстрація маршрутів для пристроїв
			deviceHandler.RegisterRoutes(r)

			// WebSocket для даних з сенсорів
			r.Get("/ws/sensors", sensorWSHandler.HandleConnection)

			// Тут реєстрація інших маршрутів...
		})
	})

	// Інформація про запуск
	log.Printf("Starting server on %s", *addr)

	// Ініціалізація HTTP-сервера
	srv := &http.Server{
		Addr:    *addr,
		Handler: r,
	}

	// Запуск сервера в окремій горутині
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Error starting server: %v", err)
		}
	}()

	// Перехоплення сигналів завершення для граційного закриття
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	// Очікування сигналу
	<-c
	log.Println("Shutting down server...")

	// Створення контексту з таймаутом для граційного завершення
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Граційне завершення сервера
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Error during server shutdown: %v", err)
	}

	log.Println("Server gracefully stopped")
}
