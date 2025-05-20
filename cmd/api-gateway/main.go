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
	"mine-detection-system/internal/infrastructure/storage"
	"mine-detection-system/internal/ports/api"
	"mine-detection-system/internal/ports/ws"
)

func main() {
	var (
		addr           = flag.String("addr", ":8080", "HTTP server address")
		dbURL          = flag.String("db", "postgres://postgres:postgres@localhost/mine_detection?sslmode=disable", "Database URL")
		minioEndpoint  = flag.String("minio-endpoint", "localhost:9000", "MinIO server endpoint")
		minioAccessKey = flag.String("minio-access-key", "minioadmin", "MinIO access key")
		minioSecretKey = flag.String("minio-secret-key", "minioadmin", "MinIO secret key")
		minioBucket    = flag.String("minio-bucket", "mine-detection", "MinIO bucket for raw data")
		minioUseSSL    = flag.Bool("minio-use-ssl", false, "Use SSL for MinIO connection")
	)
	flag.Parse()
	// Conect to BD
	db, err := sql.Open("postgres", *dbURL)
	if err != nil {
		log.Fatalf("Error connecting to database: %v", err)
	}
	defer db.Close()

	// Create a Repo
	deviceRepo := repositories.NewPostgresDeviceRepository(db)
	scanRepo := repositories.NewPostgresScanRepository(db)
	sensorDataRepo := repositories.NewPostgresSensorDataRepository(db)
	detectedObjectRepo := repositories.NewPostgresDetectedObjectRepository(db)

	geoStorage, err := storage.NewGeospatialStorage(db, *minioEndpoint, *minioAccessKey, *minioSecretKey, *minioBucket, *minioUseSSL)
	if err != nil {
		log.Fatalf("Error initializing geospatial storage: %v", err)
	}

	if err := geoStorage.InitializeDatabase(); err != nil {
		log.Printf("Warning: error initializing database schema: %v", err)
	}

	deviceService := application.NewDeviceService(deviceRepo)
	geoService := application.NewGeospatialService(geoStorage, scanRepo)
	sensorFusionService := application.NewSensorFusionService(sensorDataRepo, detectedObjectRepo, scanRepo)
	deviceHandler := api.NewDeviceHandler(deviceService)
	geoHandler := api.NewGeospatialHandler(geoService)
	sensorWSHandler := ws.NewSensorHandler(sensorFusionService, deviceService)
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(60 * time.Second))

	//@to Do: in prod chenge
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	r.Route("/api", func(r chi.Router) {
		r.Route("/v1", func(r chi.Router) {
			deviceHandler.RegisterRoutes(r)

			geoHandler.RegisterRoutes(r)

			r.Get("/ws/sensors", sensorWSHandler.HandleConnection)
		})
	})

	log.Printf("Starting server on %s", *addr)

	srv := &http.Server{
		Addr:    *addr,
		Handler: r,
	}
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Error starting server: %v", err)
		}
	}()

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	<-c
	log.Println("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Error during server shutdown: %v", err)
	}

	log.Println("Server gracefully stopped")
}
