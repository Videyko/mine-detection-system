package repositories

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"mine-detection-system/internal/domain"
	"time"
)

// PostgresSensorDataRepository реалізує інтерфейс SensorDataRepository для PostgreSQL
type PostgresSensorDataRepository struct {
	db *sql.DB
}

// NewPostgresSensorDataRepository створює новий екземпляр PostgresSensorDataRepository
func NewPostgresSensorDataRepository(db *sql.DB) *PostgresSensorDataRepository {
	return &PostgresSensorDataRepository{
		db: db,
	}
}

// Save зберігає одиничну запись даних сенсора
func (r *PostgresSensorDataRepository) Save(ctx context.Context, sensorData *domain.SensorData) error {
	query := `
		INSERT INTO sensor_data (
			id, scan_id, sensor_type, timestamp, latitude, longitude, altitude, data, quality_indicators
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`

	dataJSON, err := json.Marshal(sensorData.Data)
	if err != nil {
		return fmt.Errorf("failed to marshal sensor data: %w", err)
	}

	qualityJSON, err := json.Marshal(sensorData.QualityIndicators)
	if err != nil {
		return fmt.Errorf("failed to marshal quality indicators: %w", err)
	}

	_, err = r.db.ExecContext(
		ctx,
		query,
		sensorData.ID,
		sensorData.ScanID,
		sensorData.SensorType,
		sensorData.Timestamp,
		sensorData.Latitude,
		sensorData.Longitude,
		sensorData.Altitude,
		dataJSON,
		qualityJSON,
	)

	if err != nil {
		return fmt.Errorf("failed to save sensor data: %w", err)
	}

	return nil
}

// SaveBatch зберігає набір даних сенсорів
func (r *PostgresSensorDataRepository) SaveBatch(ctx context.Context, sensorData []*domain.SensorData) error {
	if len(sensorData) == 0 {
		return nil
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	query := `
		INSERT INTO sensor_data (
			id, scan_id, sensor_type, timestamp, latitude, longitude, altitude, data, quality_indicators
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`

	stmt, err := tx.PrepareContext(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Close()

	for i, data := range sensorData {
		dataJSON, err := json.Marshal(data.Data)
		if err != nil {
			return fmt.Errorf("failed to marshal sensor data for item %d: %w", i, err)
		}

		qualityJSON, err := json.Marshal(data.QualityIndicators)
		if err != nil {
			return fmt.Errorf("failed to marshal quality indicators for item %d: %w", i, err)
		}

		_, err = stmt.ExecContext(
			ctx,
			data.ID,
			data.ScanID,
			data.SensorType,
			data.Timestamp,
			data.Latitude,
			data.Longitude,
			data.Altitude,
			dataJSON,
			qualityJSON,
		)
		if err != nil {
			return fmt.Errorf("failed to execute statement for item %d: %w", i, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// FindBySensorType знаходить дані сенсорів за типом сенсора
func (r *PostgresSensorDataRepository) FindBySensorType(ctx context.Context, scanID uuid.UUID, sensorType string) ([]*domain.SensorData, error) {
	query := `
		SELECT id, scan_id, sensor_type, timestamp, latitude, longitude, altitude, data, quality_indicators
		FROM sensor_data
		WHERE scan_id = $1 AND sensor_type = $2
		ORDER BY timestamp
	`

	rows, err := r.db.QueryContext(ctx, query, scanID, sensorType)
	if err != nil {
		return nil, fmt.Errorf("failed to query sensor data by type: %w", err)
	}
	defer rows.Close()

	return r.scanRows(rows)
}

// FindByLocation знаходить дані сенсорів за місцем розташування
func (r *PostgresSensorDataRepository) FindByLocation(ctx context.Context, scanID uuid.UUID, latitude, longitude float64, radiusMeters float64) ([]*domain.SensorData, error) {
	query := `
		SELECT id, scan_id, sensor_type, timestamp, latitude, longitude, altitude, data, quality_indicators
		FROM sensor_data
		WHERE 
			scan_id = $1 
			AND ST_DistanceSphere(
				ST_SetSRID(ST_MakePoint(longitude, latitude), 4326),
				ST_SetSRID(ST_MakePoint($3, $2), 4326)
			) <= $4
		ORDER BY timestamp
	`

	rows, err := r.db.QueryContext(ctx, query, scanID, latitude, longitude, radiusMeters)
	if err != nil {
		return nil, fmt.Errorf("failed to query sensor data by location: %w", err)
	}
	defer rows.Close()

	return r.scanRows(rows)
}

// FindByTimeRange знаходить дані сенсорів за часовим діапазоном
func (r *PostgresSensorDataRepository) FindByTimeRange(ctx context.Context, scanID uuid.UUID, startTime, endTime time.Time) ([]*domain.SensorData, error) {
	query := `
		SELECT id, scan_id, sensor_type, timestamp, latitude, longitude, altitude, data, quality_indicators
		FROM sensor_data
		WHERE 
			scan_id = $1 
			AND timestamp >= $2
			AND timestamp <= $3
		ORDER BY timestamp
	`

	rows, err := r.db.QueryContext(ctx, query, scanID, startTime, endTime)
	if err != nil {
		return nil, fmt.Errorf("failed to query sensor data by time range: %w", err)
	}
	defer rows.Close()

	return r.scanRows(rows)
}

// FindByScanID знаходить всі дані сенсорів для конкретного сканування
func (r *PostgresSensorDataRepository) FindByScanID(ctx context.Context, scanID uuid.UUID) ([]*domain.SensorData, error) {
	query := `
		SELECT id, scan_id, sensor_type, timestamp, latitude, longitude, altitude, data, quality_indicators
		FROM sensor_data
		WHERE scan_id = $1
		ORDER BY timestamp, sensor_type
	`

	rows, err := r.db.QueryContext(ctx, query, scanID)
	if err != nil {
		return nil, fmt.Errorf("failed to query sensor data by scan ID: %w", err)
	}
	defer rows.Close()

	return r.scanRows(rows)
}

// FindLatest знаходить останні дані для кожного типу сенсора в рамках сканування
func (r *PostgresSensorDataRepository) FindLatest(ctx context.Context, scanID uuid.UUID, limit int) ([]*domain.SensorData, error) {
	query := `
		WITH latest_by_sensor AS (
			SELECT 
				id, scan_id, sensor_type, timestamp, latitude, longitude, altitude, data, quality_indicators,
				ROW_NUMBER() OVER (PARTITION BY sensor_type ORDER BY timestamp DESC) as rn
			FROM sensor_data
			WHERE scan_id = $1
		)
		SELECT id, scan_id, sensor_type, timestamp, latitude, longitude, altitude, data, quality_indicators
		FROM latest_by_sensor
		WHERE rn <= $2
		ORDER BY sensor_type, timestamp DESC
	`

	rows, err := r.db.QueryContext(ctx, query, scanID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query latest sensor data: %w", err)
	}
	defer rows.Close()

	return r.scanRows(rows)
}

// Delete видаляє дані сенсора за ID
func (r *PostgresSensorDataRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM sensor_data WHERE id = $1`

	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete sensor data: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("sensor data with id %s not found", id)
	}

	return nil
}

// DeleteByScanID видаляє всі дані сенсорів для конкретного сканування
func (r *PostgresSensorDataRepository) DeleteByScanID(ctx context.Context, scanID uuid.UUID) error {
	query := `DELETE FROM sensor_data WHERE scan_id = $1`

	_, err := r.db.ExecContext(ctx, query, scanID)
	if err != nil {
		return fmt.Errorf("failed to delete sensor data by scan ID: %w", err)
	}

	return nil
}

// scanRows - допоміжна функція для сканування рядків результату запиту
func (r *PostgresSensorDataRepository) scanRows(rows *sql.Rows) ([]*domain.SensorData, error) {
	var sensorDataList []*domain.SensorData

	for rows.Next() {
		var sensorData domain.SensorData
		var dataJSON, qualityJSON []byte

		err := rows.Scan(
			&sensorData.ID,
			&sensorData.ScanID,
			&sensorData.SensorType,
			&sensorData.Timestamp,
			&sensorData.Latitude,
			&sensorData.Longitude,
			&sensorData.Altitude,
			&dataJSON,
			&qualityJSON,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan sensor data row: %w", err)
		}

		// Розпакування JSON даних
		var data interface{}
		if len(dataJSON) > 0 {
			if err := json.Unmarshal(dataJSON, &data); err != nil {
				return nil, fmt.Errorf("failed to unmarshal sensor data: %w", err)
			}
		}
		sensorData.Data = data

		var quality interface{}
		if len(qualityJSON) > 0 {
			if err := json.Unmarshal(qualityJSON, &quality); err != nil {
				return nil, fmt.Errorf("failed to unmarshal quality indicators: %w", err)
			}
		}
		sensorData.QualityIndicators = quality

		sensorDataList = append(sensorDataList, &sensorData)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating sensor data rows: %w", err)
	}

	return sensorDataList, nil
}
