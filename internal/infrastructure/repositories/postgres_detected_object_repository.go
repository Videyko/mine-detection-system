package repositories

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"mine-detection-system/internal/domain"
)

// PostgresDetectedObjectRepository імплементує DetectedObjectRepository для PostgreSQL
type PostgresDetectedObjectRepository struct {
	db *sql.DB
}

func NewPostgresDetectedObjectRepository(db *sql.DB) *PostgresDetectedObjectRepository {
	return &PostgresDetectedObjectRepository{
		db: db,
	}
}

func (r *PostgresDetectedObjectRepository) Save(ctx context.Context, obj *domain.DetectedObject) error {
	query := `
		INSERT INTO detected_objects (
			id, scan_id, latitude, longitude, depth, object_type, confidence, danger_level, verification_status
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`

	_, err := r.db.ExecContext(
		ctx,
		query,
		obj.ID,
		obj.ScanID,
		obj.Latitude,
		obj.Longitude,
		obj.Depth,
		obj.ObjectType,
		obj.Confidence,
		obj.DangerLevel,
		obj.VerificationStatus,
	)

	return err
}

func (r *PostgresDetectedObjectRepository) FindByID(ctx context.Context, id uuid.UUID) (*domain.DetectedObject, error) {
	query := `
		SELECT id, scan_id, latitude, longitude, depth, object_type, confidence, danger_level, verification_status
		FROM detected_objects
		WHERE id = $1
	`

	var obj domain.DetectedObject

	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&obj.ID,
		&obj.ScanID,
		&obj.Latitude,
		&obj.Longitude,
		&obj.Depth,
		&obj.ObjectType,
		&obj.Confidence,
		&obj.DangerLevel,
		&obj.VerificationStatus,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.New("detected object not found")
		}
		return nil, err
	}

	return &obj, nil
}

func (r *PostgresDetectedObjectRepository) Update(ctx context.Context, obj *domain.DetectedObject) error {
	query := `
		UPDATE detected_objects
		SET scan_id = $2, latitude = $3, longitude = $4, depth = $5, object_type = $6, 
		    confidence = $7, danger_level = $8, verification_status = $9
		WHERE id = $1
	`

	result, err := r.db.ExecContext(
		ctx,
		query,
		obj.ID,
		obj.ScanID,
		obj.Latitude,
		obj.Longitude,
		obj.Depth,
		obj.ObjectType,
		obj.Confidence,
		obj.DangerLevel,
		obj.VerificationStatus,
	)

	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return errors.New("detected object not found")
	}

	return nil
}

func (r *PostgresDetectedObjectRepository) FindByScanID(ctx context.Context, scanID uuid.UUID) ([]*domain.DetectedObject, error) {
	query := `
		SELECT id, scan_id, latitude, longitude, depth, object_type, confidence, danger_level, verification_status
		FROM detected_objects
		WHERE scan_id = $1
		ORDER BY confidence DESC
	`

	rows, err := r.db.QueryContext(ctx, query, scanID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var objects []*domain.DetectedObject

	for rows.Next() {
		var obj domain.DetectedObject

		err := rows.Scan(
			&obj.ID,
			&obj.ScanID,
			&obj.Latitude,
			&obj.Longitude,
			&obj.Depth,
			&obj.ObjectType,
			&obj.Confidence,
			&obj.DangerLevel,
			&obj.VerificationStatus,
		)
		if err != nil {
			return nil, err
		}

		objects = append(objects, &obj)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return objects, nil
}

// FindByCoordinates знаходить всі виявлені об'єкти в заданому радіусі від координат
func (r *PostgresDetectedObjectRepository) FindByCoordinates(ctx context.Context, lat, lon float64, radius float64) ([]*domain.DetectedObject, error) {
	// Використовуємо PostGIS для пошуку за координатами в межах радіусу
	query := `
		SELECT id, scan_id, latitude, longitude, depth, object_type, confidence, danger_level, verification_status
		FROM detected_objects
		WHERE ST_DWithin(
			ST_SetSRID(ST_MakePoint(longitude, latitude), 4326)::geography,
			ST_SetSRID(ST_MakePoint($1, $2), 4326)::geography,
			$3
		)
		ORDER BY confidence DESC
	`

	rows, err := r.db.QueryContext(ctx, query, lon, lat, radius)
	if err != nil {
		return nil, fmt.Errorf("failed to query detected objects by coordinates: %w", err)
	}
	defer rows.Close()

	var objects []*domain.DetectedObject
	for rows.Next() {
		var obj domain.DetectedObject
		err := rows.Scan(
			&obj.ID,
			&obj.ScanID,
			&obj.Latitude,
			&obj.Longitude,
			&obj.Depth,
			&obj.ObjectType,
			&obj.Confidence,
			&obj.DangerLevel,
			&obj.VerificationStatus,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan detected object row: %w", err)
		}
		objects = append(objects, &obj)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating detected object rows: %w", err)
	}

	return objects, nil
}

// FindByLocation знаходить об'єкти в заданому радіусі (альтернативна реалізація)
func (r *PostgresDetectedObjectRepository) FindByLocation(ctx context.Context, latitude, longitude float64, radiusMeters float64) ([]*domain.DetectedObject, error) {
	query := `
		SELECT id, scan_id, latitude, longitude, depth, object_type, confidence, danger_level, verification_status
		FROM detected_objects
		WHERE ST_DistanceSphere(
			ST_SetSRID(ST_MakePoint(longitude, latitude), 4326),
			ST_SetSRID(ST_MakePoint($2, $1), 4326)
		) <= $3
		ORDER BY confidence DESC
	`

	rows, err := r.db.QueryContext(ctx, query, latitude, longitude, radiusMeters)
	if err != nil {
		return nil, fmt.Errorf("failed to query detected objects by location: %w", err)
	}
	defer rows.Close()

	var objects []*domain.DetectedObject

	for rows.Next() {
		var obj domain.DetectedObject

		err := rows.Scan(
			&obj.ID,
			&obj.ScanID,
			&obj.Latitude,
			&obj.Longitude,
			&obj.Depth,
			&obj.ObjectType,
			&obj.Confidence,
			&obj.DangerLevel,
			&obj.VerificationStatus,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan detected object row: %w", err)
		}

		objects = append(objects, &obj)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating detected object rows: %w", err)
	}

	return objects, nil
}

func (r *PostgresDetectedObjectRepository) UpdateVerificationStatus(ctx context.Context, id uuid.UUID, status domain.VerificationStatus) error {
	query := `
		UPDATE detected_objects
		SET verification_status = $2
		WHERE id = $1
	`

	result, err := r.db.ExecContext(ctx, query, id, status)
	if err != nil {
		return fmt.Errorf("failed to update verification status: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return errors.New("detected object not found")
	}

	return nil
}

func (r *PostgresDetectedObjectRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM detected_objects WHERE id = $1`

	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete detected object: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return errors.New("detected object not found")
	}

	return nil
}
