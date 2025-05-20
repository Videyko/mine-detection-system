package repositories

import (
	"context"
	"database/sql"
	"errors"
	"github.com/google/uuid"
	"mine-detection-system/internal/domain"
)

// PostgresDeviceRepository імплементує DeviceRepository для PostgreSQL
type PostgresDeviceRepository struct {
	db *sql.DB
}

// NewPostgresDeviceRepository створює новий екземпляр PostgresDeviceRepository
func NewPostgresDeviceRepository(db *sql.DB) *PostgresDeviceRepository {
	return &PostgresDeviceRepository{
		db: db,
	}
}

func (r *PostgresDeviceRepository) Save(ctx context.Context, device *domain.Device) error {
	query := `
        INSERT INTO devices (id, device_type, serial_number, config_json, status, created_at, last_connection_at)
        VALUES ($1, $2, $3, $4, $5, $6, $7)
    `

	_, err := r.db.ExecContext(
		ctx,
		query,
		device.ID,
		device.DeviceType,
		device.SerialNumber,
		device.Configuration,
		device.Status,
		device.CreatedAt,
		device.LastConnectionAt,
	)

	return err
}

func (r *PostgresDeviceRepository) FindByID(ctx context.Context, id uuid.UUID) (*domain.Device, error) {
	query := `
        SELECT id, device_type, serial_number, config_json, status, created_at, last_connection_at
        FROM devices
        WHERE id = $1
    `

	var device domain.Device
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&device.ID,
		&device.DeviceType,
		&device.SerialNumber,
		&device.Configuration,
		&device.Status,
		&device.CreatedAt,
		&device.LastConnectionAt,
	)

	if err == sql.ErrNoRows {
		return nil, errors.New("device not found")
	}

	if err != nil {
		return nil, err
	}

	return &device, nil
}

// FindAll шукає пристрої за фільтрами
func (r *PostgresDeviceRepository) FindAll(ctx context.Context, filters map[string]interface{}) ([]*domain.Device, error) {
	query := `
        SELECT id, device_type, serial_number, config_json, status, created_at, last_connection_at
        FROM devices
        WHERE 1=1
    `

	var args []interface{}
	argIndex := 1

	// Додавання фільтрів
	if serialNumber, ok := filters["serial_number"]; ok {
		query += " AND serial_number = $" + string(argIndex)
		args = append(args, serialNumber)
		argIndex++
	}

	if deviceType, ok := filters["device_type"]; ok {
		query += " AND device_type = $" + string(argIndex)
		args = append(args, deviceType)
		argIndex++
	}

	if status, ok := filters["status"]; ok {
		query += " AND status = $" + string(argIndex)
		args = append(args, status)
		argIndex++
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var devices []*domain.Device
	for rows.Next() {
		var device domain.Device
		if err := rows.Scan(
			&device.ID,
			&device.DeviceType,
			&device.SerialNumber,
			&device.Configuration,
			&device.Status,
			&device.CreatedAt,
			&device.LastConnectionAt,
		); err != nil {
			return nil, err
		}
		devices = append(devices, &device)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return devices, nil
}

func (r *PostgresDeviceRepository) Update(ctx context.Context, device *domain.Device) error {
	query := `
        UPDATE devices
        SET device_type = $1, serial_number = $2, config_json = $3, status = $4, last_connection_at = $5
        WHERE id = $6
    `

	result, err := r.db.ExecContext(
		ctx,
		query,
		device.DeviceType,
		device.SerialNumber,
		device.Configuration,
		device.Status,
		device.LastConnectionAt,
		device.ID,
	)

	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return errors.New("device not found")
	}

	return nil
}

func (r *PostgresDeviceRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM devices WHERE id = $1`

	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return errors.New("device not found")
	}

	return nil
}
