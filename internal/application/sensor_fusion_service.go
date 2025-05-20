package application

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"mine-detection-system/internal/domain"
	"mine-detection-system/internal/infrastructure/repositories"
	"mine-detection-system/internal/ports"
	"mine-detection-system/pkg/fusion"
	"time"
)

type SensorFusionService struct {
	sensorDataRepo     ports.SensorDataRepository
	detectedObjectRepo ports.DetectedObjectRepository
	scanRepo           ports.ScanRepository
}

func NewSensorFusionService(sensorDataRepo *repositories.PostgresSensorDataRepository, detectedObjectRepo ports.DetectedObjectRepository, scanRepo *repositories.PostgresScanRepository) *SensorFusionService {
	return &SensorFusionService{
		sensorDataRepo:     sensorDataRepo,
		detectedObjectRepo: detectedObjectRepo,
		scanRepo:           scanRepo,
	}
}

func (s *SensorFusionService) ProcessSensorData(ctx context.Context, scanID uuid.UUID, sensorType string, data []byte, metadata map[string]interface{}) error {

	if _, err := s.scanRepo.FindByID(ctx, scanID); err != nil {
		return err
	}

	latitude, ok := metadata["latitude"].(float64)
	if !ok {
		return errors.New("метадані не містять коректного поля latitude")
	}

	longitude, ok := metadata["longitude"].(float64)
	if !ok {
		return errors.New("метадані не містять коректного поля longitude")
	}

	altitude, ok := metadata["altitude"].(float64)
	if !ok {
		return errors.New("метадані не містять коректного поля altitude")
	}

	qualityIndicators, ok := metadata["quality"]
	if !ok {
		return errors.New("метадані не містять поля quality")
	}

	processedData, err := s.processSensorTypeData(sensorType, data)
	if err != nil {
		return err
	}

	sensorData := &domain.SensorData{
		ID:                uuid.New(),
		ScanID:            scanID,
		SensorType:        sensorType,
		Timestamp:         time.Now(),
		Latitude:          latitude,
		Longitude:         longitude,
		Altitude:          altitude,
		Data:              processedData,
		QualityIndicators: qualityIndicators,
	}

	return s.sensorDataRepo.SaveBatch(ctx, []*domain.SensorData{sensorData})
}

func (s *SensorFusionService) FuseAndDetect(ctx context.Context, scanID uuid.UUID, regionID string) ([]*domain.DetectedObject, error) {

	lidarData, err := s.sensorDataRepo.FindBySensorType(ctx, scanID, "lidar")
	if err != nil {
		return nil, err
	}

	magneticData, err := s.sensorDataRepo.FindBySensorType(ctx, scanID, "magnetic")
	if err != nil {
		return nil, err
	}

	acousticData, err := s.sensorDataRepo.FindBySensorType(ctx, scanID, "acoustic")
	if err != nil {
		return nil, err
	}

	// Conect the data
	detector := fusion.NewDetector()
	detections, err := detector.FuseAndDetect(lidarData, magneticData, acousticData)
	if err != nil {
		return nil, err
	}

	var detectedObjects []*domain.DetectedObject
	for _, detection := range detections {
		detectedObject := &domain.DetectedObject{
			ID:                 uuid.New(),
			ScanID:             scanID,
			Latitude:           detection.Latitude,
			Longitude:          detection.Longitude,
			Depth:              detection.Depth,
			ObjectType:         detection.ObjectType,
			Confidence:         detection.Confidence,
			DangerLevel:        detection.DangerLevel,
			VerificationStatus: domain.VerificationStatusUnverified,
		}

		if err := s.detectedObjectRepo.Save(ctx, detectedObject); err != nil {
			return nil, err
		}

		detectedObjects = append(detectedObjects, detectedObject)
	}

	return detectedObjects, nil
}

func (s *SensorFusionService) processSensorTypeData(sensorType string, data []byte) (interface{}, error) {
	switch sensorType {
	case "lidar":
		return fusion.ProcessLidarData(data)
	case "magnetic":
		return fusion.ProcessMagneticData(data)
	case "acoustic":
		return fusion.ProcessAcousticData(data)
	default:
		return nil, errors.New("unsupported sensor type")
	}
}
