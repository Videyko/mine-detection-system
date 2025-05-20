package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"mine-detection-system/internal/domain"
)

// GeospatialStorage забезпечує зберігання та доступ до геопросторових даних
type GeospatialStorage struct {
	db          *sql.DB
	minioClient *minio.Client
	bucketName  string
}

// NewGeospatialStorage створює новий екземпляр GeospatialStorage
func NewGeospatialStorage(db *sql.DB, minioEndpoint, minioAccessKey, minioSecretKey, minioBucket string, useSSL bool) (*GeospatialStorage, error) {
	// Ініціалізація MinIO клієнта
	minioClient, err := minio.New(minioEndpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(minioAccessKey, minioSecretKey, ""),
		Secure: useSSL,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to initialize MinIO client: %w", err)
	}

	// Перевірка наявності бакета і створення його, якщо не існує
	exists, err := minioClient.BucketExists(context.Background(), minioBucket)
	if err != nil {
		return nil, fmt.Errorf("failed to check if bucket exists: %w", err)
	}

	if !exists {
		err = minioClient.MakeBucket(context.Background(), minioBucket, minio.MakeBucketOptions{})
		if err != nil {
			return nil, fmt.Errorf("failed to create bucket: %w", err)
		}
	}

	return &GeospatialStorage{
		db:          db,
		minioClient: minioClient,
		bucketName:  minioBucket,
	}, nil
}

// Ініціалізація TimescaleDB та PostGIS
func (s *GeospatialStorage) InitializeDatabase() error {
	// Перевірка та встановлення розширення PostGIS
	_, err := s.db.Exec("CREATE EXTENSION IF NOT EXISTS postgis")
	if err != nil {
		return fmt.Errorf("failed to create PostGIS extension: %w", err)
	}

	// Перевірка та встановлення розширення TimescaleDB
	_, err = s.db.Exec("CREATE EXTENSION IF NOT EXISTS timescaledb")
	if err != nil {
		return fmt.Errorf("failed to create TimescaleDB extension: %w", err)
	}

	// Створення таблиці sensor_data з геопросторовою підтримкою, якщо вона не існує
	_, err = s.db.Exec(`
		CREATE TABLE IF NOT EXISTS sensor_data (
			id UUID PRIMARY KEY,
			scan_id UUID NOT NULL,
			sensor_type TEXT NOT NULL,
			timestamp TIMESTAMPTZ NOT NULL,
			location GEOGRAPHY(POINT, 4326),
			altitude FLOAT,
			data JSONB,
			quality_indicators JSONB
		)
	`)
	if err != nil {
		return fmt.Errorf("failed to create sensor_data table: %w", err)
	}

	// Перетворення таблиці sensor_data на гіпертаблицю TimescaleDB
	_, err = s.db.Exec(`
		SELECT create_hypertable('sensor_data', 'timestamp', 
			chunk_time_interval => INTERVAL '1 hour',
			if_not_exists => TRUE)
	`)
	if err != nil {
		return fmt.Errorf("failed to create hypertable: %w", err)
	}

	// Створення просторового індексу
	_, err = s.db.Exec(`
		CREATE INDEX IF NOT EXISTS sensor_data_location_idx 
		ON sensor_data USING GIST (location)
	`)
	if err != nil {
		return fmt.Errorf("failed to create spatial index: %w", err)
	}

	// Створення індексу для sensor_type для швидкого пошуку за типом сенсора
	_, err = s.db.Exec(`
		CREATE INDEX IF NOT EXISTS sensor_data_type_idx 
		ON sensor_data (sensor_type)
	`)
	if err != nil {
		return fmt.Errorf("failed to create sensor type index: %w", err)
	}

	// Створення складеного індексу для швидкого пошуку за scan_id та timestamp
	_, err = s.db.Exec(`
		CREATE INDEX IF NOT EXISTS sensor_data_scan_time_idx 
		ON sensor_data (scan_id, timestamp)
	`)
	if err != nil {
		return fmt.Errorf("failed to create scan time index: %w", err)
	}

	return nil
}

// SaveSensorData зберігає дані сенсорів у TimescaleDB
func (s *GeospatialStorage) SaveSensorData(ctx context.Context, data *domain.SensorData) error {
	// Створення POINT географії з координат
	query := `
		INSERT INTO sensor_data (id, scan_id, sensor_type, timestamp, location, altitude, data, quality_indicators)
		VALUES ($1, $2, $3, $4, ST_SetSRID(ST_MakePoint($5, $6), 4326), $7, $8, $9)
	`

	dataJSON, err := json.Marshal(data.Data)
	if err != nil {
		return fmt.Errorf("failed to marshal data: %w", err)
	}

	qualityJSON, err := json.Marshal(data.QualityIndicators)
	if err != nil {
		return fmt.Errorf("failed to marshal quality indicators: %w", err)
	}

	_, err = s.db.ExecContext(
		ctx,
		query,
		data.ID,
		data.ScanID,
		data.SensorType,
		data.Timestamp,
		data.Longitude, // ST_MakePoint приймає (lon, lat) у цьому порядку
		data.Latitude,
		data.Altitude,
		dataJSON,
		qualityJSON,
	)

	if err != nil {
		return fmt.Errorf("failed to insert sensor data: %w", err)
	}

	return nil
}

// SaveRawScanData зберігає необроблені дані сканування в MinIO
func (s *GeospatialStorage) SaveRawScanData(ctx context.Context, scanID uuid.UUID, sensorType string, data io.Reader, size int64) (string, error) {
	// Формування об'єктного ключа за допомогою timestamp для унікальності
	objectKey := fmt.Sprintf("%s/%s/%s.bin", scanID, sensorType, time.Now().Format("20060102-150405.999"))

	// Збереження даних у MinIO
	_, err := s.minioClient.PutObject(ctx, s.bucketName, objectKey, data, size, minio.PutObjectOptions{
		ContentType: "application/octet-stream",
		UserMetadata: map[string]string{
			"scan-id":      scanID.String(),
			"sensor-type":  sensorType,
			"created-time": time.Now().Format(time.RFC3339),
		},
	})

	if err != nil {
		return "", fmt.Errorf("failed to save raw scan data: %w", err)
	}

	return objectKey, nil
}

// GetRawScanData отримує необроблені дані сканування з MinIO
func (s *GeospatialStorage) GetRawScanData(ctx context.Context, objectKey string) (io.ReadCloser, error) {
	obj, err := s.minioClient.GetObject(ctx, s.bucketName, objectKey, minio.GetObjectOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get raw scan data: %w", err)
	}

	return obj, nil
}

// FindSensorDataInArea знаходить дані сенсорів у заданій географічній області
func (s *GeospatialStorage) FindSensorDataInArea(ctx context.Context, scanID uuid.UUID, sensorType string, centerLat, centerLon, radiusMeters float64) ([]*domain.SensorData, error) {
	query := `
		SELECT id, scan_id, sensor_type, timestamp, 
			ST_Y(location::geometry) as latitude, 
			ST_X(location::geometry) as longitude, 
			altitude, data, quality_indicators
		FROM sensor_data
		WHERE scan_id = $1
		AND sensor_type = $2
		AND ST_DWithin(
			location,
			ST_SetSRID(ST_MakePoint($3, $4), 4326),
			$5
		)
		ORDER BY timestamp
	`

	rows, err := s.db.QueryContext(ctx, query, scanID, sensorType, centerLon, centerLat, radiusMeters)
	if err != nil {
		return nil, fmt.Errorf("failed to query sensor data in area: %w", err)
	}
	defer rows.Close()

	var results []*domain.SensorData
	for rows.Next() {
		var data domain.SensorData
		var dataJSON, qualityJSON []byte

		err := rows.Scan(
			&data.ID,
			&data.ScanID,
			&data.SensorType,
			&data.Timestamp,
			&data.Latitude,
			&data.Longitude,
			&data.Altitude,
			&dataJSON,
			&qualityJSON,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan sensor data row: %w", err)
		}

		// Розпакування JSON полів
		if err := json.Unmarshal(dataJSON, &data.Data); err != nil {
			log.Printf("Warning: failed to unmarshal data JSON: %v", err)
		}

		if err := json.Unmarshal(qualityJSON, &data.QualityIndicators); err != nil {
			log.Printf("Warning: failed to unmarshal quality indicators JSON: %v", err)
		}

		results = append(results, &data)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating sensor data rows: %w", err)
	}

	return results, nil
}

// GetTemporalAggregation отримує агреговані дані для часового ряду
func (s *GeospatialStorage) GetTemporalAggregation(
	ctx context.Context,
	scanID uuid.UUID,
	sensorType string,
	startTime, endTime time.Time,
	timeInterval string,
) ([]map[string]interface{}, error) {
	// Перевірка правильності параметра timeInterval для запобігання SQL-ін'єкціям
	validIntervals := map[string]bool{
		"1 minute":   true,
		"5 minutes":  true,
		"10 minutes": true,
		"15 minutes": true,
		"30 minutes": true,
		"1 hour":     true,
		"1 day":      true,
	}

	if !validIntervals[timeInterval] {
		return nil, errors.New("invalid time interval")
	}

	// SQL запит для агрегації даних за часовими інтервалами
	query := fmt.Sprintf(`
		SELECT 
			time_bucket('%s', timestamp) AS bucket,
			COUNT(*) AS reading_count,
			AVG(altitude) AS avg_altitude,
			AVG(ST_Y(location::geometry)) AS avg_latitude,
			AVG(ST_X(location::geometry)) AS avg_longitude
		FROM sensor_data
		WHERE scan_id = $1
		AND sensor_type = $2
		AND timestamp BETWEEN $3 AND $4
		GROUP BY bucket
		ORDER BY bucket
	`, timeInterval)

	rows, err := s.db.QueryContext(ctx, query, scanID, sensorType, startTime, endTime)
	if err != nil {
		return nil, fmt.Errorf("failed to query temporal aggregation: %w", err)
	}
	defer rows.Close()

	var results []map[string]interface{}
	for rows.Next() {
		var bucket time.Time
		var count int
		var avgAltitude, avgLatitude, avgLongitude float64

		if err := rows.Scan(&bucket, &count, &avgAltitude, &avgLatitude, &avgLongitude); err != nil {
			return nil, fmt.Errorf("failed to scan temporal aggregation row: %w", err)
		}

		result := map[string]interface{}{
			"time":          bucket,
			"reading_count": count,
			"avg_altitude":  avgAltitude,
			"avg_latitude":  avgLatitude,
			"avg_longitude": avgLongitude,
		}

		results = append(results, result)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating temporal aggregation rows: %w", err)
	}

	return results, nil
}

// GetSpatialHeatmap генерує дані теплової карти для візуалізації
func (s *GeospatialStorage) GetSpatialHeatmap(
	ctx context.Context,
	scanID uuid.UUID,
	sensorType string,
	startTime, endTime time.Time,
	gridSize float64,
) ([]map[string]interface{}, error) {
	// Запит для створення гексагональної сітки та агрегації даних
	query := `
		WITH grid AS (
			SELECT h3_cell_to_boundary(
				h3_lat_lng_to_cell(
					ST_Y(location::geometry), 
					ST_X(location::geometry), 
					$5
				)
			) AS hexagon,
			COUNT(*) AS point_count,
			AVG(ST_Y(location::geometry)) AS center_lat,
			AVG(ST_X(location::geometry)) AS center_lon
			FROM sensor_data
			WHERE scan_id = $1
			AND sensor_type = $2
			AND timestamp BETWEEN $3 AND $4
			GROUP BY h3_lat_lng_to_cell(
				ST_Y(location::geometry), 
				ST_X(location::geometry), 
				$5
			)
		)
		SELECT 
			center_lat,
			center_lon,
			point_count,
			ST_AsGeoJSON(hexagon) AS geometry
		FROM grid
		ORDER BY point_count DESC
	`

	// Визначення правильного рівня H3 залежно від gridSize
	// H3 використовує рівні від 0 (найгрубіший) до 15 (найдетальніший)
	// Для простоти, використовуємо приблизну відповідність
	h3Resolution := 9 // Значення за замовчуванням, ~170 метрів
	if gridSize <= 50 {
		h3Resolution = 10 // ~65 метрів
	} else if gridSize <= 500 {
		h3Resolution = 8 // ~460 метрів
	} else if gridSize <= 1000 {
		h3Resolution = 7 // ~1.2 км
	}

	rows, err := s.db.QueryContext(ctx, query, scanID, sensorType, startTime, endTime, h3Resolution)
	if err != nil {
		return nil, fmt.Errorf("failed to query spatial heatmap: %w", err)
	}
	defer rows.Close()

	var results []map[string]interface{}
	for rows.Next() {
		var centerLat, centerLon float64
		var pointCount int
		var geometryJSON string

		if err := rows.Scan(&centerLat, &centerLon, &pointCount, &geometryJSON); err != nil {
			return nil, fmt.Errorf("failed to scan spatial heatmap row: %w", err)
		}

		var geometry map[string]interface{}
		if err := json.Unmarshal([]byte(geometryJSON), &geometry); err != nil {
			return nil, fmt.Errorf("failed to parse geometry JSON: %w", err)
		}

		result := map[string]interface{}{
			"center_lat":  centerLat,
			"center_lon":  centerLon,
			"point_count": pointCount,
			"geometry":    geometry,
		}

		results = append(results, result)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating spatial heatmap rows: %w", err)
	}

	return results, nil
}

// ListRawScanDataKeys повертає список ключів до сирих даних сканування
func (s *GeospatialStorage) ListRawScanDataKeys(ctx context.Context, scanID uuid.UUID, sensorType string) ([]string, error) {
	prefix := fmt.Sprintf("%s/%s/", scanID, sensorType)

	// Створення каналу для отримання об'єктів
	objectCh := s.minioClient.ListObjects(ctx, s.bucketName, minio.ListObjectsOptions{
		Prefix:    prefix,
		Recursive: true,
	})

	var keys []string
	for object := range objectCh {
		if object.Err != nil {
			return nil, fmt.Errorf("error listing objects: %w", object.Err)
		}
		keys = append(keys, object.Key)
	}

	return keys, nil
}
