package ports

import (
	"context"
	"github.com/google/uuid"
	"mine-detection-system/internal/domain"
	"time"
)

// DeviceRepository визначає методи для роботи з пристроями
type DeviceRepository interface {
	Save(ctx context.Context, device *domain.Device) error
	FindByID(ctx context.Context, id uuid.UUID) (*domain.Device, error)
	FindAll(ctx context.Context, filters map[string]interface{}) ([]*domain.Device, error)
	Update(ctx context.Context, device *domain.Device) error
	Delete(ctx context.Context, id uuid.UUID) error
}

// MissionRepository визначає методи для роботи з місіями
type MissionRepository interface {
	Save(ctx context.Context, mission *domain.Mission) error
	FindByID(ctx context.Context, id uuid.UUID) (*domain.Mission, error)
	FindAll(ctx context.Context, filters map[string]interface{}) ([]*domain.Mission, error)
	Update(ctx context.Context, mission *domain.Mission) error
	Delete(ctx context.Context, id uuid.UUID) error
}

// ScanRepository визначає методи для роботи зі скануваннями
type ScanRepository interface {
	Save(ctx context.Context, scan *domain.Scan) error
	FindByID(ctx context.Context, id uuid.UUID) (*domain.Scan, error)
	FindByMissionID(ctx context.Context, missionID uuid.UUID) ([]*domain.Scan, error)
	FindByDeviceID(ctx context.Context, deviceID uuid.UUID) ([]*domain.Scan, error)
	Update(ctx context.Context, scan *domain.Scan) error
}

// SensorDataRepository визначає методи для роботи з даними сенсорів
type SensorDataRepository interface {
	// Основні методи збереження
	Save(ctx context.Context, sensorData *domain.SensorData) error
	SaveBatch(ctx context.Context, data []*domain.SensorData) error

	// Методи пошуку (відповідають реалізації PostgresSensorDataRepository)
	FindByScanID(ctx context.Context, scanID uuid.UUID) ([]*domain.SensorData, error)
	FindBySensorType(ctx context.Context, scanID uuid.UUID, sensorType string) ([]*domain.SensorData, error)
	FindByLocation(ctx context.Context, scanID uuid.UUID, latitude, longitude float64, radiusMeters float64) ([]*domain.SensorData, error)
	FindByTimeRange(ctx context.Context, scanID uuid.UUID, startTime, endTime time.Time) ([]*domain.SensorData, error)
	FindLatest(ctx context.Context, scanID uuid.UUID, limit int) ([]*domain.SensorData, error)

	// Методи видалення
	Delete(ctx context.Context, id uuid.UUID) error
	DeleteByScanID(ctx context.Context, scanID uuid.UUID) error
}

// DetectedObjectRepository визначає методи для роботи з виявленими об'єктами
type DetectedObjectRepository interface {
	Save(ctx context.Context, obj *domain.DetectedObject) error
	FindByID(ctx context.Context, id uuid.UUID) (*domain.DetectedObject, error)
	FindByScanID(ctx context.Context, scanID uuid.UUID) ([]*domain.DetectedObject, error)
	FindByCoordinates(ctx context.Context, lat, lon float64, radius float64) ([]*domain.DetectedObject, error)
	FindByLocation(ctx context.Context, latitude, longitude float64, radiusMeters float64) ([]*domain.DetectedObject, error)
	Update(ctx context.Context, obj *domain.DetectedObject) error
	UpdateVerificationStatus(ctx context.Context, id uuid.UUID, status domain.VerificationStatus) error
	Delete(ctx context.Context, id uuid.UUID) error
}
