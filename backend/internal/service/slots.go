package service

import (
	"context"
	"fmt"
	"log"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/sto/crm/backend/internal/domain"
	"github.com/sto/crm/backend/internal/store"
)

// hashPassword хеширует пароль алгоритмом bcrypt.
func hashPassword(password string) (string, error) {
	h, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("не удалось захешировать пароль: %w", err)
	}
	return string(h), nil
}

// comparePassword сверяет пароль с хешем.
func comparePassword(hash, password string) error {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
}

// LogNotifier — заглушка уведомлений: пишет SMS/Email в консоль.
// Заменяется на реальный SMS-шлюз без изменения кода вызывающей стороны.
type LogNotifier struct {
	Logger *log.Logger
}

// NewLogNotifier создаёт заглушку уведомлений.
func NewLogNotifier(logger *log.Logger) *LogNotifier {
	return &LogNotifier{Logger: logger}
}

// SendSMS имитирует отправку SMS.
func (n *LogNotifier) SendSMS(phone, message string) {
	n.Logger.Printf("[SMS] -> %s: %s", phone, message)
}

// SendEmail имитирует отправку письма.
func (n *LogNotifier) SendEmail(email, subject, body string) {
	n.Logger.Printf("[EMAIL] -> %s | %s: %s", email, subject, body)
}

// SlotService — расчёт интеллектуального календаря: цветовой шкалы загрузки дня.
type SlotService struct {
	orders   *store.OrderRepo
	capacity int
	start    string
	end      string
}

// NewSlotService создаёт сервис календаря загрузки.
func NewSlotService(orders *store.OrderRepo, capacity int, start, end string) *SlotService {
	return &SlotService{orders: orders, capacity: capacity, start: start, end: end}
}

// DaySlots рассчитывает загруженность филиала на дату по часовым слотам.
func (s *SlotService) DaySlots(ctx context.Context, branchID int, date time.Time) (*domain.DaySlots, error) {
	counts, err := s.orders.CountsByBranchAndTime(ctx, branchID, date)
	if err != nil {
		return nil, err
	}

	startMin, err := parseClockToMinutes(s.start)
	if err != nil {
		return nil, err
	}
	endMin, err := parseClockToMinutes(s.end)
	if err != nil {
		return nil, err
	}

	slots := make([]domain.Slot, 0, endMin-startMin)
	peak := 0
	for m := startMin; m < endMin; m += 60 {
		hour := m / 60
		minute := m % 60
		key := fmt.Sprintf("%02d:%02d", hour, minute)
		cnt := counts[key]
		// Часовой слот: заказы, начатые в этот час, занимают бокс на этот час.
		load := 0
		if s.capacity > 0 {
			load = cnt * 100 / s.capacity
		}
		if cnt > peak {
			peak = cnt
		}
		slots = append(slots, domain.Slot{
			Time:      key,
			Orders:    cnt,
			Capacity:  s.capacity,
			LoadPct:   load,
			LoadLevel: LoadLevel(load),
		})
	}

	// Загрузка дня определяется пиковым часом: именно в нём риск перегрузки.
	overall := 0
	if s.capacity > 0 {
		overall = peak * 100 / s.capacity
	}
	total := 0
	for _, c := range counts {
		total += c
	}

	return &domain.DaySlots{
		Date:      date.Format("2006-01-02"),
		BranchID:  branchID,
		Total:     total,
		Capacity:  s.capacity,
		LoadPct:   overall,
		LoadLevel: LoadLevel(overall),
		Slots:     slots,
	}, nil
}

// LoadLevel переводит процент загрузки в цветовой уровень шкалы.
// До 30% — зелёный, 30–70% — жёлтый, свыше 70% — красный.
func LoadLevel(pct int) string {
	switch {
	case pct <= 30:
		return "green"
	case pct <= 70:
		return "yellow"
	default:
		return "red"
	}
}

// parseClockToMinutes переводит строку HH:MM в количество минут от полуночи.
func parseClockToMinutes(s string) (int, error) {
	t, err := time.Parse("15:04", s)
	if err != nil {
		return 0, fmt.Errorf("некорректное время %q: %w", s, err)
	}
	return t.Hour()*60 + t.Minute(), nil
}
