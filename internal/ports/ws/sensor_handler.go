package ws

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"log"
	"mine-detection-system/internal/application"
	"net/http"
	"sync"
	"time"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // В продакшені тут має бути перевірка походження запиту
	},
}

// SensorHandler обробляє WebSocket з'єднання для даних з сенсорів
type SensorHandler struct {
	sensorService *application.SensorFusionService
	deviceService *application.DeviceService
	connections   map[uuid.UUID]*websocket.Conn
	connectionsMu sync.Mutex
}

// NewSensorHandler створює новий SensorHandler
func NewSensorHandler(
	sensorService *application.SensorFusionService,
	deviceService *application.DeviceService,
) *SensorHandler {
	return &SensorHandler{
		sensorService: sensorService,
		deviceService: deviceService,
		connections:   make(map[uuid.UUID]*websocket.Conn),
	}
}

// HandleConnection оброблює WebSocket з'єднання
func (h *SensorHandler) HandleConnection(w http.ResponseWriter, r *http.Request) {
	// Аутентифікація та авторизація
	deviceID, err := authenticateDevice(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Оновлення статусу пристрою
	ctx := r.Context()
	err = h.deviceService.UpdateDeviceStatus(ctx, deviceID, "active")
	if err != nil {
		http.Error(w, "Error updating device status", http.StatusInternalServerError)
		return
	}

	// Оновлення WebSocket з'єднання
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("Error upgrading connection: %v", err)
		return
	}

	// Реєстрація з'єднання
	h.connectionsMu.Lock()
	h.connections[deviceID] = conn
	h.connectionsMu.Unlock()

	// Запуск горутин для обробки повідомлень
	go h.handleMessages(ctx, deviceID, conn)
}

// handleMessages обробляє повідомлення від пристрою
func (h *SensorHandler) handleMessages(ctx context.Context, deviceID uuid.UUID, conn *websocket.Conn) {
	defer func() {
		conn.Close()

		h.connectionsMu.Lock()
		delete(h.connections, deviceID)
		h.connectionsMu.Unlock()

		// Оновлення статусу пристрою при від'єднанні
		err := h.deviceService.UpdateDeviceStatus(context.Background(), deviceID, "inactive")
		if err != nil {
			log.Printf("Error updating device status: %v", err)
		}
	}()

	// Налаштування ping/pong для підтримки з'єднання
	conn.SetPingHandler(func(string) error {
		if err := conn.WriteControl(websocket.PongMessage, []byte{}, time.Now().Add(time.Second)); err != nil {
			return err
		}
		return nil
	})

	// Цикл обробки повідомлень
	for {
		messageType, p, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket error: %v", err)
			}
			break
		}

		// Обробка тільки бінарних повідомлень та текстових JSON
		switch messageType {
		case websocket.BinaryMessage:
			h.handleBinaryMessage(ctx, deviceID, p)
		case websocket.TextMessage:
			h.handleTextMessage(ctx, deviceID, p)
		}
	}
}

// handleBinaryMessage обробляє бінарні повідомлення з даними сенсорів
func (h *SensorHandler) handleBinaryMessage(ctx context.Context, deviceID uuid.UUID, data []byte) {
	// Розбір заголовка бінарного повідомлення
	if len(data) < 8 {
		log.Printf("Invalid binary message format")
		return
	}

	// Парсинг бінарного заголовка (магічне число, тип пакету тощо)
	if data[0] != 0xAA || data[1] != 0x55 {
		log.Printf("Invalid magic number in binary message")
		return
	}

	// Тип пакету
	packetType := data[3]

	// Парсинг метаданих та даних залежно від типу пакету
	switch packetType {
	case 0x01: // Пакет з даними ЛІДАР
		scanID, err := extractScanID(data)
		if err != nil {
			log.Printf("Error extracting scan ID: %v", err)
			return
		}

		metadata, dataStart := extractMetadata(data)
		sensorData := data[dataStart:]

		err = h.sensorService.ProcessSensorData(ctx, scanID, "lidar", sensorData, metadata)
		if err != nil {
			log.Printf("Error processing LIDAR data: %v", err)
		}

	case 0x02: // Пакет з даними магнітометра
		// Аналогічно обробці ЛІДАР...

	case 0x03: // Пакет з акустичними даними
		// Аналогічно обробці ЛІДАР...

	default:
		log.Printf("Unknown packet type: %d", packetType)
	}
}

// handleTextMessage обробляє текстові повідомлення у форматі JSON
func (h *SensorHandler) handleTextMessage(ctx context.Context, deviceID uuid.UUID, data []byte) {
	var message map[string]interface{}
	if err := json.Unmarshal(data, &message); err != nil {
		log.Printf("Error unmarshaling JSON: %v", err)
		return
	}

	messageType, ok := message["type"].(string)
	if !ok {
		log.Printf("Missing message type")
		return
	}

	switch messageType {
	case "heartbeat":
		// Обробка heartbeat-повідомлень
		h.handleHeartbeat(ctx, deviceID, message)

	case "scan_start":
		// Обробка початку сканування
		h.handleScanStart(ctx, deviceID, message)

	case "scan_end":
		// Обробка завершення сканування
		h.handleScanEnd(ctx, deviceID, message)

	default:
		log.Printf("Unknown message type: %s", messageType)
	}
}

// authenticateDevice аутентифікує пристрій за токеном у запиті
func authenticateDevice(r *http.Request) (uuid.UUID, error) {
	token := r.URL.Query().Get("token")
	if token == "" {
		return uuid.Nil, errors.New("missing authentication token")
	}

	// В реальності тут має бути перевірка токена у базі даних або через JWT
	// Для прикладу просто використаємо токен як ID пристрою
	return uuid.Parse(token)
}

// extractScanID витягує ID сканування з бінарного пакету
func extractScanID(data []byte) (uuid.UUID, error) {
	// Припускаємо, що ID сканування знаходиться у байтах 8-23
	if len(data) < 24 {
		return uuid.Nil, errors.New("data too short to contain scan ID")
	}

	return uuid.FromBytes(data[8:24])
}

// extractMetadata витягує метадані з бінарного пакету
func extractMetadata(data []byte) (map[string]interface{}, int) {
	// Спрощена реалізація для прикладу
	// В реальному сценарії потрібно розбирати TLV структуру
	metadata := map[string]interface{}{
		"latitude":  float64(int32(data[24])<<24|int32(data[25])<<16|int32(data[26])<<8|int32(data[27])) / 1000000.0,
		"longitude": float64(int32(data[28])<<24|int32(data[29])<<16|int32(data[30])<<8|int32(data[31])) / 1000000.0,
		"altitude":  float64(int32(data[32])<<24|int32(data[33])<<16|int32(data[34])<<8|int32(data[35])) / 100.0,
		"quality": map[string]interface{}{
			"signalStrength": int(data[36]),
		},
	}

	return metadata, 40 // Повертаємо початок області даних після метаданих
}

// Допоміжні методи для обробки повідомлень

func (h *SensorHandler) handleHeartbeat(ctx context.Context, deviceID uuid.UUID, message map[string]interface{}) {
	// Оновлення останнього з'єднання для пристрою
	err := h.deviceService.UpdateDeviceStatus(ctx, deviceID, "active")
	if err != nil {
		log.Printf("Error updating device status: %v", err)
	}

	// Відправка відповіді на heartbeat
	response := map[string]interface{}{
		"type": "heartbeat_ack",
		"time": time.Now().Unix(),
	}

	h.sendMessage(deviceID, response)
}

func (h *SensorHandler) handleScanStart(ctx context.Context, deviceID uuid.UUID, message map[string]interface{}) {
	// Логіка обробки початку сканування...
}

func (h *SensorHandler) handleScanEnd(ctx context.Context, deviceID uuid.UUID, message map[string]interface{}) {
	// Логіка обробки завершення сканування...
}

// sendMessage відправляє повідомлення пристрою
func (h *SensorHandler) sendMessage(deviceID uuid.UUID, message interface{}) {
	h.connectionsMu.Lock()
	conn, exists := h.connections[deviceID]
	h.connectionsMu.Unlock()

	if !exists {
		log.Printf("Device %s not connected", deviceID)
		return
	}

	data, err := json.Marshal(message)
	if err != nil {
		log.Printf("Error marshaling message: %v", err)
		return
	}

	if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
		log.Printf("Error sending message: %v", err)
	}
}
