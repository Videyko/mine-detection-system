package ports

import (
	"context"
	"io"
	"time"

	"github.com/google/uuid"
	"mine-detection-system/internal/domain"
)

// GeospatialStorage визначає інтерфейс для роботи з геопросторовими даними
type GeospatialStorage interface {
	// Ініціалізація геопросторової бази даних з необхідними розширеннями
	InitializeDatabase() error

	// Зберігання та отримання даних сенсорів
	SaveSensorData(ctx context.Context, data *domain.SensorData) error
	FindSensorDataInArea(ctx context.Context, scanID uuid.UUID, sensorType string, centerLat, centerLon, radiusMeters float64) ([]*domain.SensorData, error)

	// Робота з необробленими даними сканування
	SaveRawScanData(ctx context.Context, scanID uuid.UUID, sensorType string, data io.Reader, size int64) (string, error)
	GetRawScanData(ctx context.Context, objectKey string) (io.ReadCloser, error)
	ListRawScanDataKeys(ctx context.Context, scanID uuid.UUID, sensorType string) ([]string, error)

	// Аналітичні функції
	GetTemporalAggregation(ctx context.Context, scanID uuid.UUID, sensorType string, startTime, endTime time.Time, timeInterval string) ([]map[string]interface{}, error)
	GetSpatialHeatmap(ctx context.Context, scanID uuid.UUID, sensorType string, startTime, endTime time.Time, gridSize float64) ([]map[string]interface{}, error)
}
