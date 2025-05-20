package domain

import (
	"github.com/google/uuid"
	"time"
)

// Enums для статусів
type DeviceStatus string
type MissionStatus string
type ScanStatus string
type VerificationStatus string

const (
	// Статуси пристроїв
	DeviceStatusActive      DeviceStatus = "active"
	DeviceStatusInactive    DeviceStatus = "inactive"
	DeviceStatusMaintenance DeviceStatus = "maintenance"

	// Статуси місій
	MissionStatusPlanned   MissionStatus = "planned"
	MissionStatusActive    MissionStatus = "active"
	MissionStatusCompleted MissionStatus = "completed"
	MissionStatusAborted   MissionStatus = "aborted"

	// Статуси сканувань
	ScanStatusInProgress ScanStatus = "in_progress"
	ScanStatusCompleted  ScanStatus = "completed"
	ScanStatusFailed     ScanStatus = "failed"

	// Статуси верифікації виявлених об'єктів
	VerificationStatusUnverified VerificationStatus = "unverified"
	VerificationStatusConfirmed  VerificationStatus = "confirmed"
	VerificationStatusDismissed  VerificationStatus = "dismissed"
)

// Device представляє фізичний пристрій для виявлення мін
type Device struct {
	ID               uuid.UUID    `json:"id"`
	DeviceType       string       `json:"device_type"`
	SerialNumber     string       `json:"serial_number"`
	Configuration    interface{}  `json:"configuration"`
	Status           DeviceStatus `json:"status"`
	CreatedAt        time.Time    `json:"created_at"`
	LastConnectionAt time.Time    `json:"last_connection_at"`
}

// Mission представляє операцію з розмінування
type Mission struct {
	ID          uuid.UUID     `json:"id"`
	Name        string        `json:"name"`
	Description string        `json:"description"`
	Boundaries  GeoJSON       `json:"boundaries"`
	Status      MissionStatus `json:"status"`
	StartDate   time.Time     `json:"start_date"`
	EndDate     *time.Time    `json:"end_date"`
	Priority    int           `json:"priority"`
}

// Scan представляє окремий сеанс сканування
type Scan struct {
	ID        uuid.UUID   `json:"id"`
	MissionID uuid.UUID   `json:"mission_id"`
	DeviceID  uuid.UUID   `json:"device_id"`
	StartTime time.Time   `json:"start_time"`
	EndTime   *time.Time  `json:"end_time"`
	ScanType  string      `json:"scan_type"`
	Status    ScanStatus  `json:"status"`
	Metadata  interface{} `json:"metadata"`
}

// SensorData представляє агреговані дані з сенсорів
type SensorData struct {
	ID                uuid.UUID   `json:"id"`
	ScanID            uuid.UUID   `json:"scan_id"`
	SensorType        string      `json:"sensor_type"`
	Timestamp         time.Time   `json:"timestamp"`
	Latitude          float64     `json:"latitude"`
	Longitude         float64     `json:"longitude"`
	Altitude          float64     `json:"altitude"`
	Data              interface{} `json:"data"`
	QualityIndicators interface{} `json:"quality_indicators"`
}

// DetectedObject представляє потенційну міну
type DetectedObject struct {
	ID                 uuid.UUID          `json:"id"`
	ScanID             uuid.UUID          `json:"scan_id"`
	Latitude           float64            `json:"latitude"`
	Longitude          float64            `json:"longitude"`
	Depth              float64            `json:"depth"`
	ObjectType         string             `json:"object_type"`
	Confidence         float64            `json:"confidence"`
	DangerLevel        int                `json:"danger_level"`
	VerificationStatus VerificationStatus `json:"verification_status"`
}

// GeoJSON представляє геопросторові дані
type GeoJSON map[string]interface{}
