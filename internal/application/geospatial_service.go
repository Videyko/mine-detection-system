package application

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mine-detection-system/internal/infrastructure/repositories"
	"time"

	"github.com/google/uuid"
	"mine-detection-system/internal/domain"
	"mine-detection-system/internal/ports"
)

type GeospatialService struct {
	geoStorage ports.GeospatialStorage
	scanRepo   ports.ScanRepository
}

func NewGeospatialService(geoStorage ports.GeospatialStorage, scanRepo *repositories.PostgresScanRepository) *GeospatialService {
	return &GeospatialService{
		geoStorage: geoStorage,
		scanRepo:   scanRepo,
	}
}

func (s *GeospatialService) SaveRawScanData(ctx context.Context, scanID uuid.UUID, sensorType string, data io.Reader, size int64) (string, error) {

	scan, err := s.scanRepo.FindByID(ctx, scanID)
	if err != nil {
		return "", fmt.Errorf("scan not found: %w", err)
	}

	if scan.Status != domain.ScanStatusInProgress {
		return "", errors.New("cannot save data for inactive scan")
	}

	return s.geoStorage.SaveRawScanData(ctx, scanID, sensorType, data, size)
}

func (s *GeospatialService) GetRawScanData(ctx context.Context, objectKey string) (io.ReadCloser, error) {
	return s.geoStorage.GetRawScanData(ctx, objectKey)
}

func (s *GeospatialService) GetSensorDataAroundPoint(
	ctx context.Context,
	scanID uuid.UUID,
	sensorType string,
	latitude, longitude, radiusMeters float64,
) ([]*domain.SensorData, error) {
	return s.geoStorage.FindSensorDataInArea(ctx, scanID, sensorType, latitude, longitude, radiusMeters)
}

func (s *GeospatialService) GetSpatialHeatmap(
	ctx context.Context,
	scanID uuid.UUID,
	sensorType string,
	startTime, endTime time.Time,
	gridSize float64,
) ([]map[string]interface{}, error) {
	_, err := s.scanRepo.FindByID(ctx, scanID)
	if err != nil {
		return nil, fmt.Errorf("scan not found: %w", err)
	}

	return s.geoStorage.GetSpatialHeatmap(ctx, scanID, sensorType, startTime, endTime, gridSize)
}

func (s *GeospatialService) GetTemporalAnalysis(
	ctx context.Context,
	scanID uuid.UUID,
	sensorType string,
	startTime, endTime time.Time,
	timeInterval string,
) ([]map[string]interface{}, error) {
	_, err := s.scanRepo.FindByID(ctx, scanID)
	if err != nil {
		return nil, fmt.Errorf("scan not found: %w", err)
	}

	return s.geoStorage.GetTemporalAggregation(ctx, scanID, sensorType, startTime, endTime, timeInterval)
}

func (s *GeospatialService) GetAvailableRawDataFiles(ctx context.Context, scanID uuid.UUID, sensorType string) ([]string, error) {
	_, err := s.scanRepo.FindByID(ctx, scanID)
	if err != nil {
		return nil, fmt.Errorf("scan not found: %w", err)
	}

	return s.geoStorage.ListRawScanDataKeys(ctx, scanID, sensorType)
}

func (s *GeospatialService) GenerateReportData(ctx context.Context, scanID uuid.UUID) (map[string]interface{}, error) {
	scan, err := s.scanRepo.FindByID(ctx, scanID)
	if err != nil {
		return nil, fmt.Errorf("scan not found: %w", err)
	}

	if scan.Status != domain.ScanStatusCompleted {
		return nil, errors.New("report can be generated only for completed scans")
	}

	endTime := time.Now()
	if scan.EndTime != nil {
		endTime = *scan.EndTime
	}

	lidarData, _ := s.geoStorage.GetTemporalAggregation(ctx, scanID, "lidar", scan.StartTime, endTime, "5 minutes")
	magneticData, _ := s.geoStorage.GetTemporalAggregation(ctx, scanID, "magnetic", scan.StartTime, endTime, "5 minutes")
	acousticData, _ := s.geoStorage.GetTemporalAggregation(ctx, scanID, "acoustic", scan.StartTime, endTime, "5 minutes")

	report := map[string]interface{}{
		"scan_id":    scanID,
		"start_time": scan.StartTime,
		"end_time":   endTime,
		"duration":   endTime.Sub(scan.StartTime).String(),
		"scan_type":  scan.ScanType,
		"sensor_data": map[string]interface{}{
			"lidar":    lidarData,
			"magnetic": magneticData,
			"acoustic": acousticData,
		},
	}

	return report, nil
}
