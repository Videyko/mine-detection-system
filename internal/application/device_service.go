package application

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"mine-detection-system/internal/domain"
	"mine-detection-system/internal/ports"
	"time"
)

// DeviceService відповідає за бізнес-логіку роботи з пристроями
type DeviceService struct {
	deviceRepo ports.DeviceRepository
}

// NewDeviceService створює новий екземпляр DeviceService
func NewDeviceService(deviceRepo ports.DeviceRepository) *DeviceService {
	return &DeviceService{
		deviceRepo: deviceRepo,
	}
}

// RegisterDevice реєструє новий пристрій в системі
func (s *DeviceService) RegisterDevice(ctx context.Context, deviceType, serialNumber string, config interface{}) (*domain.Device, error) {
	// Перевірка, чи пристрій вже існує
	devices, err := s.deviceRepo.FindAll(ctx, map[string]interface{}{
		"serial_number": serialNumber,
	})
	if err != nil {
		return nil, err
	}
	if len(devices) > 0 {
		return nil, errors.New("device with this serial number already exists")
	}

	// Створення нового пристрою
	device := &domain.Device{
		ID:               uuid.New(),
		DeviceType:       deviceType,
		SerialNumber:     serialNumber,
		Configuration:    config,
		Status:           domain.DeviceStatusInactive,
		CreatedAt:        time.Now(),
		LastConnectionAt: time.Now(),
	}

	// Збереження пристрою
	if err := s.deviceRepo.Save(ctx, device); err != nil {
		return nil, err
	}

	return device, nil
}

// UpdateDeviceStatus оновлює статус пристрою
func (s *DeviceService) UpdateDeviceStatus(ctx context.Context, deviceID uuid.UUID, status domain.DeviceStatus) error {
	device, err := s.deviceRepo.FindByID(ctx, deviceID)
	if err != nil {
		return err
	}

	device.Status = status
	device.LastConnectionAt = time.Now()

	return s.deviceRepo.Update(ctx, device)
}

// GetDeviceByID отримує пристрій за ID
func (s *DeviceService) GetDeviceByID(ctx context.Context, deviceID uuid.UUID) (*domain.Device, error) {
	return s.deviceRepo.FindByID(ctx, deviceID)
}

// ListDevices отримує список всіх пристроїв з можливістю фільтрації
func (s *DeviceService) ListDevices(ctx context.Context, filters map[string]interface{}) ([]*domain.Device, error) {
	return s.deviceRepo.FindAll(ctx, filters)
}

// UpdateDeviceConfiguration оновлює конфігурацію пристрою
func (s *DeviceService) UpdateDeviceConfiguration(ctx context.Context, deviceID uuid.UUID, config interface{}) error {
	device, err := s.deviceRepo.FindByID(ctx, deviceID)
	if err != nil {
		return err
	}

	device.Configuration = config
	device.LastConnectionAt = time.Now()

	return s.deviceRepo.Update(ctx, device)
}
