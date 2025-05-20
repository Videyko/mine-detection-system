package api

import (
	"encoding/json"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"mine-detection-system/internal/application"
	"mine-detection-system/internal/domain"
	"net/http"
)

// DeviceHandler обробляє HTTP-запити, пов'язані з пристроями
type DeviceHandler struct {
	deviceService *application.DeviceService
}

// NewDeviceHandler створює новий DeviceHandler
func NewDeviceHandler(deviceService *application.DeviceService) *DeviceHandler {
	return &DeviceHandler{
		deviceService: deviceService,
	}
}

// RegisterRoutes реєструє маршрути для DeviceHandler
func (h *DeviceHandler) RegisterRoutes(r chi.Router) {
	r.Route("/devices", func(r chi.Router) {
		r.Get("/", h.ListDevices)
		r.Post("/", h.CreateDevice)
		r.Get("/{id}", h.GetDevice)
		r.Put("/{id}/status", h.UpdateDeviceStatus)
		r.Put("/{id}/config", h.UpdateDeviceConfig)
	})
}

// ListDevices обробляє GET /devices
func (h *DeviceHandler) ListDevices(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Отримання фільтрів з query parameters
	filters := make(map[string]interface{})
	if status := r.URL.Query().Get("status"); status != "" {
		filters["status"] = status
	}
	if deviceType := r.URL.Query().Get("type"); deviceType != "" {
		filters["device_type"] = deviceType
	}

	devices, err := h.deviceService.ListDevices(ctx, filters)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(devices); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

// CreateDevice обробляє POST /devices
func (h *DeviceHandler) CreateDevice(w http.ResponseWriter, r *http.Request) {
	var request struct {
		DeviceType    string      `json:"device_type"`
		SerialNumber  string      `json:"serial_number"`
		Configuration interface{} `json:"configuration"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	device, err := h.deviceService.RegisterDevice(ctx, request.DeviceType, request.SerialNumber, request.Configuration)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(device); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

// GetDevice обробляє GET /devices/{id}
func (h *DeviceHandler) GetDevice(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		http.Error(w, "Invalid device ID", http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	device, err := h.deviceService.GetDeviceByID(ctx, id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(device); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

// UpdateDeviceStatus обробляє PUT /devices/{id}/status
func (h *DeviceHandler) UpdateDeviceStatus(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		http.Error(w, "Invalid device ID", http.StatusBadRequest)
		return
	}

	var request struct {
		Status string `json:"status"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	err = h.deviceService.UpdateDeviceStatus(ctx, id, domain.DeviceStatus(request.Status))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// UpdateDeviceConfig обробляє PUT /devices/{id}/config
func (h *DeviceHandler) UpdateDeviceConfig(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		http.Error(w, "Invalid device ID", http.StatusBadRequest)
		return
	}

	var config interface{}
	if err := json.NewDecoder(r.Body).Decode(&config); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	err = h.deviceService.UpdateDeviceConfiguration(ctx, id, config)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
