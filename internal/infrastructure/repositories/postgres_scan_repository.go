package repositories

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"mine-detection-system/internal/domain"
)

// PostgresScanRepository реалізує інтерфейс ScanRepository для PostgreSQL
type PostgresScanRepository struct {
	db *sql.DB
}

// NewPostgresScanRepository створює новий екземпляр PostgresScanRepository
func NewPostgresScanRepository(db *sql.DB) *PostgresScanRepository {
	return &PostgresScanRepository{
		db: db,
	}
}

// FindByID знаходить сканування за ID
func (r *PostgresScanRepository) FindByID(ctx context.Context, id uuid.UUID) (*domain.Scan, error) {
	query := `
		SELECT id, mission_id, device_id, start_time, end_time, scan_type, status, metadata
		FROM scans
		WHERE id = $1
	`

	var scan domain.Scan
	var endTimeSQL sql.NullTime
	var metadataJSON []byte

	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&scan.ID,
		&scan.MissionID,
		&scan.DeviceID,
		&scan.StartTime,
		&endTimeSQL,
		&scan.ScanType,
		&scan.Status,
		&metadataJSON,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.New("scan not found")
		}
		return nil, fmt.Errorf("failed to find scan: %w", err)
	}

	if endTimeSQL.Valid {
		endTime := endTimeSQL.Time
		scan.EndTime = &endTime
	}

	// Розпакування метаданих з JSON
	if len(metadataJSON) > 0 {
		var metadata map[string]interface{}
		if err := json.Unmarshal(metadataJSON, &metadata); err != nil {
			return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
		}
		scan.Metadata = metadata
	}

	return &scan, nil
}

// Save зберігає нове сканування
func (r *PostgresScanRepository) Save(ctx context.Context, scan *domain.Scan) error {
	query := `
		INSERT INTO scans (id, mission_id, device_id, start_time, end_time, scan_type, status, metadata)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`

	var endTimeSQL sql.NullTime
	if scan.EndTime != nil {
		endTimeSQL = sql.NullTime{
			Time:  *scan.EndTime,
			Valid: true,
		}
	}

	// Пакування метаданих у JSON
	var metadataJSON []byte
	if scan.Metadata != nil {
		var err error
		metadataJSON, err = json.Marshal(scan.Metadata)
		if err != nil {
			return fmt.Errorf("failed to marshal metadata: %w", err)
		}
	}

	_, err := r.db.ExecContext(
		ctx,
		query,
		scan.ID,
		scan.MissionID,
		scan.DeviceID,
		scan.StartTime,
		endTimeSQL,
		scan.ScanType,
		scan.Status,
		metadataJSON,
	)

	if err != nil {
		return fmt.Errorf("failed to save scan: %w", err)
	}

	return nil
}

// Update оновлює існуюче сканування
func (r *PostgresScanRepository) Update(ctx context.Context, scan *domain.Scan) error {
	query := `
		UPDATE scans
		SET mission_id = $2, device_id = $3, start_time = $4, end_time = $5, scan_type = $6, status = $7, metadata = $8
		WHERE id = $1
	`

	var endTimeSQL sql.NullTime
	if scan.EndTime != nil {
		endTimeSQL = sql.NullTime{
			Time:  *scan.EndTime,
			Valid: true,
		}
	}

	// Пакування метаданих у JSON
	var metadataJSON []byte
	if scan.Metadata != nil {
		var err error
		metadataJSON, err = json.Marshal(scan.Metadata)
		if err != nil {
			return fmt.Errorf("failed to marshal metadata: %w", err)
		}
	}

	result, err := r.db.ExecContext(
		ctx,
		query,
		scan.ID,
		scan.MissionID,
		scan.DeviceID,
		scan.StartTime,
		endTimeSQL,
		scan.ScanType,
		scan.Status,
		metadataJSON,
	)

	if err != nil {
		return fmt.Errorf("failed to update scan: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return errors.New("scan not found")
	}

	return nil
}

// FindByMissionID знаходить всі сканування для конкретної місії
func (r *PostgresScanRepository) FindByMissionID(ctx context.Context, missionID uuid.UUID) ([]*domain.Scan, error) {
	query := `
		SELECT id, mission_id, device_id, start_time, end_time, scan_type, status, metadata
		FROM scans
		WHERE mission_id = $1
		ORDER BY start_time DESC
	`

	return r.executeQueryAndScanRows(ctx, query, missionID)
}

// FindActiveByDeviceID знаходить активні сканування для конкретного пристрою
func (r *PostgresScanRepository) FindActiveByDeviceID(ctx context.Context, deviceID uuid.UUID) (*domain.Scan, error) {
	query := `
		SELECT id, mission_id, device_id, start_time, end_time, scan_type, status, metadata
		FROM scans
		WHERE device_id = $1 AND status = 'in_progress'
		ORDER BY start_time DESC
		LIMIT 1
	`

	var scan domain.Scan
	var endTimeSQL sql.NullTime
	var metadataJSON []byte

	err := r.db.QueryRowContext(ctx, query, deviceID).Scan(
		&scan.ID,
		&scan.MissionID,
		&scan.DeviceID,
		&scan.StartTime,
		&endTimeSQL,
		&scan.ScanType,
		&scan.Status,
		&metadataJSON,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // Немає активних сканувань
		}
		return nil, fmt.Errorf("failed to find active scan: %w", err)
	}

	if endTimeSQL.Valid {
		endTime := endTimeSQL.Time
		scan.EndTime = &endTime
	}

	// Розпакування метаданих з JSON
	if len(metadataJSON) > 0 {
		var metadata map[string]interface{}
		if err := json.Unmarshal(metadataJSON, &metadata); err != nil {
			return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
		}
		scan.Metadata = metadata
	}

	return &scan, nil
}

// FindByDeviceID знаходить всі сканування для конкретного пристрою
func (r *PostgresScanRepository) FindByDeviceID(ctx context.Context, deviceID uuid.UUID) ([]*domain.Scan, error) {
	query := `
		SELECT id, mission_id, device_id, start_time, end_time, scan_type, status, metadata
		FROM scans
		WHERE device_id = $1
		ORDER BY start_time DESC
	`

	return r.executeQueryAndScanRows(ctx, query, deviceID)
}

// FindByStatus знаходить сканування за статусом
func (r *PostgresScanRepository) FindByStatus(ctx context.Context, status domain.ScanStatus) ([]*domain.Scan, error) {
	query := `
		SELECT id, mission_id, device_id, start_time, end_time, scan_type, status, metadata
		FROM scans
		WHERE status = $1
		ORDER BY start_time DESC
	`

	return r.executeQueryAndScanRows(ctx, query, status)
}

// UpdateStatus оновлює статус сканування
func (r *PostgresScanRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status domain.ScanStatus) error {
	query := `
		UPDATE scans
		SET status = $1
		WHERE id = $2
	`

	result, err := r.db.ExecContext(ctx, query, status, id)
	if err != nil {
		return fmt.Errorf("failed to update scan status: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return errors.New("scan not found")
	}

	return nil
}

// Delete видаляє сканування
func (r *PostgresScanRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM scans WHERE id = $1`

	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete scan: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return errors.New("scan not found")
	}

	return nil
}

// executeQueryAndScanRows - допоміжна функція для виконання запитів і сканування рядків
func (r *PostgresScanRepository) executeQueryAndScanRows(ctx context.Context, query string, args ...interface{}) ([]*domain.Scan, error) {
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to execute query: %w", err)
	}
	defer rows.Close()

	var scans []*domain.Scan

	for rows.Next() {
		var scan domain.Scan
		var endTimeSQL sql.NullTime
		var metadataJSON []byte

		err := rows.Scan(
			&scan.ID,
			&scan.MissionID,
			&scan.DeviceID,
			&scan.StartTime,
			&endTimeSQL,
			&scan.ScanType,
			&scan.Status,
			&metadataJSON,
		)

		if err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		if endTimeSQL.Valid {
			endTime := endTimeSQL.Time
			scan.EndTime = &endTime
		}

		// Розпакування метаданих з JSON
		if len(metadataJSON) > 0 {
			var metadata map[string]interface{}
			if err := json.Unmarshal(metadataJSON, &metadata); err != nil {
				return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
			}
			scan.Metadata = metadata
		}

		scans = append(scans, &scan)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	return scans, nil
}
