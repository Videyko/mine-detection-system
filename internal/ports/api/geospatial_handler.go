package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"mine-detection-system/internal/application"
)

// GeospatialHandler обробляє HTTP-запити, пов'язані з геопросторовими даними
type GeospatialHandler struct {
	geoService *application.GeospatialService
}

// NewGeospatialHandler створює новий GeospatialHandler
func NewGeospatialHandler(geoService *application.GeospatialService) *GeospatialHandler {
	return &GeospatialHandler{
		geoService: geoService,
	}
}

// RegisterRoutes реєструє маршрути для GeospatialHandler
func (h *GeospatialHandler) RegisterRoutes(r chi.Router) {
	r.Route("/geo", func(r chi.Router) {
		r.Route("/scans/{scanID}", func(r chi.Router) {
			r.Get("/heatmap", h.GetSpatialHeatmap)
			r.Get("/timeline", h.GetTemporalAnalysis)
			r.Get("/sensors", h.GetSensorDataAroundPoint)
			r.Get("/raw-data", h.ListRawDataFiles)
			r.Get("/raw-data/{key}", h.GetRawData)
			r.Post("/raw-data", h.UploadRawData)
			r.Get("/report", h.GenerateReport)
		})
	})
}

// GetSpatialHeatmap обробляє запит на отримання даних теплової карти
func (h *GeospatialHandler) GetSpatialHeatmap(w http.ResponseWriter, r *http.Request) {
	scanIDStr := chi.URLParam(r, "scanID")
	scanID, err := uuid.Parse(scanIDStr)
	if err != nil {
		http.Error(w, "Invalid scan ID", http.StatusBadRequest)
		return
	}

	// Отримання параметрів запиту
	sensorType := r.URL.Query().Get("type")
	if sensorType == "" {
		http.Error(w, "Sensor type is required", http.StatusBadRequest)
		return
	}

	startTimeStr := r.URL.Query().Get("start")
	endTimeStr := r.URL.Query().Get("end")
	gridSizeStr := r.URL.Query().Get("grid_size")

	// Парсинг параметрів
	startTime := time.Now().Add(-24 * time.Hour) // За замовчуванням - 24 години назад
	if startTimeStr != "" {
		parsedTime, err := time.Parse(time.RFC3339, startTimeStr)
		if err == nil {
			startTime = parsedTime
		}
	}

	endTime := time.Now() // За замовчуванням - поточний час
	if endTimeStr != "" {
		parsedTime, err := time.Parse(time.RFC3339, endTimeStr)
		if err == nil {
			endTime = parsedTime
		}
	}

	gridSize := 100.0 // За замовчуванням - 100 метрів
	if gridSizeStr != "" {
		parsedSize, err := strconv.ParseFloat(gridSizeStr, 64)
		if err == nil && parsedSize > 0 {
			gridSize = parsedSize
		}
	}

	// Отримання даних теплової карти
	ctx := r.Context()
	heatmapData, err := h.geoService.GetSpatialHeatmap(ctx, scanID, sensorType, startTime, endTime, gridSize)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Повернення результату
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(heatmapData); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

// GetTemporalAnalysis обробляє запит на отримання часового аналізу даних
func (h *GeospatialHandler) GetTemporalAnalysis(w http.ResponseWriter, r *http.Request) {
	scanIDStr := chi.URLParam(r, "scanID")
	scanID, err := uuid.Parse(scanIDStr)
	if err != nil {
		http.Error(w, "Invalid scan ID", http.StatusBadRequest)
		return
	}

	// Отримання параметрів запиту
	sensorType := r.URL.Query().Get("type")
	if sensorType == "" {
		http.Error(w, "Sensor type is required", http.StatusBadRequest)
		return
	}

	startTimeStr := r.URL.Query().Get("start")
	endTimeStr := r.URL.Query().Get("end")
	interval := r.URL.Query().Get("interval")

	// Парсинг параметрів
	startTime := time.Now().Add(-24 * time.Hour) // За замовчуванням - 24 години назад
	if startTimeStr != "" {
		parsedTime, err := time.Parse(time.RFC3339, startTimeStr)
		if err == nil {
			startTime = parsedTime
		}
	}

	endTime := time.Now() // За замовчуванням - поточний час
	if endTimeStr != "" {
		parsedTime, err := time.Parse(time.RFC3339, endTimeStr)
		if err == nil {
			endTime = parsedTime
		}
	}

	// Валідація інтервалу
	if interval == "" {
		interval = "5 minutes" // За замовчуванням
	}

	validIntervals := map[string]bool{
		"1 minute":   true,
		"5 minutes":  true,
		"10 minutes": true,
		"15 minutes": true,
		"30 minutes": true,
		"1 hour":     true,
		"1 day":      true,
	}

	if !validIntervals[interval] {
		http.Error(w, "Invalid interval", http.StatusBadRequest)
		return
	}

	// Отримання даних часового аналізу
	ctx := r.Context()
	timelineData, err := h.geoService.GetTemporalAnalysis(ctx, scanID, sensorType, startTime, endTime, interval)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Повернення результату
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(timelineData); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

// GetSensorDataAroundPoint обробляє запит на отримання даних сенсорів навколо точки
func (h *GeospatialHandler) GetSensorDataAroundPoint(w http.ResponseWriter, r *http.Request) {
	scanIDStr := chi.URLParam(r, "scanID")
	scanID, err := uuid.Parse(scanIDStr)
	if err != nil {
		http.Error(w, "Invalid scan ID", http.StatusBadRequest)
		return
	}

	// Отримання параметрів запиту
	sensorType := r.URL.Query().Get("type")
	if sensorType == "" {
		http.Error(w, "Sensor type is required", http.StatusBadRequest)
		return
	}

	latStr := r.URL.Query().Get("lat")
	lonStr := r.URL.Query().Get("lon")
	radiusStr := r.URL.Query().Get("radius")

	if latStr == "" || lonStr == "" {
		http.Error(w, "Latitude and longitude are required", http.StatusBadRequest)
		return
	}

	// Парсинг параметрів
	lat, err := strconv.ParseFloat(latStr, 64)
	if err != nil {
		http.Error(w, "Invalid latitude", http.StatusBadRequest)
		return
	}

	lon, err := strconv.ParseFloat(lonStr, 64)
	if err != nil {
		http.Error(w, "Invalid longitude", http.StatusBadRequest)
		return
	}

	radius := 100.0 // За замовчуванням - 100 метрів
	if radiusStr != "" {
		parsedRadius, err := strconv.ParseFloat(radiusStr, 64)
		if err == nil && parsedRadius > 0 {
			radius = parsedRadius
		}
	}

	// Отримання даних сенсорів
	ctx := r.Context()
	sensorData, err := h.geoService.GetSensorDataAroundPoint(ctx, scanID, sensorType, lat, lon, radius)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Повернення результату
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(sensorData); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

// ListRawDataFiles обробляє запит на отримання списку файлів необроблених даних
func (h *GeospatialHandler) ListRawDataFiles(w http.ResponseWriter, r *http.Request) {
	scanIDStr := chi.URLParam(r, "scanID")
	scanID, err := uuid.Parse(scanIDStr)
	if err != nil {
		http.Error(w, "Invalid scan ID", http.StatusBadRequest)
		return
	}

	// Отримання типу сенсора
	sensorType := r.URL.Query().Get("type")
	if sensorType == "" {
		sensorType = "all" // За замовчуванням - всі типи
	}

	// Отримання списку файлів
	ctx := r.Context()
	files, err := h.geoService.GetAvailableRawDataFiles(ctx, scanID, sensorType)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Повернення результату
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(files); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func (h *GeospatialHandler) GetRawData(w http.ResponseWriter, r *http.Request) {
	key := chi.URLParam(r, "key")
	if key == "" {
		http.Error(w, "Object key is required", http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	dataReader, err := h.geoService.GetRawScanData(ctx, key)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer dataReader.Close()

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", key))

	// Копіювання даних у відповідь
	if _, err := io.Copy(w, dataReader); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func (h *GeospatialHandler) UploadRawData(w http.ResponseWriter, r *http.Request) {
	scanIDStr := chi.URLParam(r, "scanID")
	scanID, err := uuid.Parse(scanIDStr)
	if err != nil {
		http.Error(w, "Invalid scan ID", http.StatusBadRequest)
		return
	}

	sensorType := r.URL.Query().Get("type")
	if sensorType == "" {
		http.Error(w, "Sensor type is required", http.StatusBadRequest)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 100*1024*1024) // Обмеження 100 МБ
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		http.Error(w, "File too large", http.StatusBadRequest)
		return
	}

	file, handler, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "Error retrieving file", http.StatusBadRequest)
		return
	}
	defer file.Close()

	ctx := r.Context()
	objectKey, err := h.geoService.SaveRawScanData(ctx, scanID, sensorType, file, handler.Size)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	response := map[string]string{
		"object_key": objectKey,
	}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func (h *GeospatialHandler) GenerateReport(w http.ResponseWriter, r *http.Request) {
	scanIDStr := chi.URLParam(r, "scanID")
	scanID, err := uuid.Parse(scanIDStr)
	if err != nil {
		http.Error(w, "Invalid scan ID", http.StatusBadRequest)
		return
	}
	ctx := r.Context()
	reportData, err := h.geoService.GenerateReportData(ctx, scanID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(reportData); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}
