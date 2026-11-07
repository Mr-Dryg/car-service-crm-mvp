package service

import (
	"testing"

	"github.com/sto/crm/backend/internal/domain"
)

// TestLoadLevel проверяет пороги цветовой шкалы загрузки филиала.
func TestLoadLevel(t *testing.T) {
	cases := []struct {
		pct  int
		want string
	}{
		{0, "green"},
		{30, "green"},
		{31, "yellow"},
		{70, "yellow"},
		{71, "red"},
		{100, "red"},
		{175, "red"},
	}
	for _, c := range cases {
		if got := LoadLevel(c.pct); got != c.want {
			t.Errorf("LoadLevel(%d) = %q, ожидалось %q", c.pct, got, c.want)
		}
	}
}

// TestNormalizePhone проверяет нормализацию номера телефона.
func TestNormalizePhone(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"+7 (999) 123-45-67", "+79991234567"},
		{"89991234567", "89991234567"},
		{"  +7 999 123 45 67  ", "+79991234567"},
		{"", ""},
	}
	for _, c := range cases {
		if got := normalizePhone(c.in); got != c.want {
			t.Errorf("normalizePhone(%q) = %q, ожидалось %q", c.in, got, c.want)
		}
	}
}

// TestParseDateTime проверяет разбор даты и времени записи.
func TestParseDateTime(t *testing.T) {
	date, clock, err := parseDateTime("2026-10-25", "10:00")
	if err != nil {
		t.Fatalf("ожидалась ошибка nil, получено %v", err)
	}
	if date != "2026-10-25" {
		t.Errorf("дата = %q, ожидалось 2026-10-25", date)
	}
	if clock != "10:00" {
		t.Errorf("время = %q, ожидалось 10:00", clock)
	}

	if _, _, err := parseDateTime("25.10.2026", "10:00"); err == nil {
		t.Error("ожидалась ошибка для некорректной даты")
	}
	if _, _, err := parseDateTime("2026-10-25", "25:00"); err == nil {
		t.Error("ожидалась ошибка для некорректного времени")
	}
}

// TestStatusTransitions проверяет допустимость переходов статусов.
func TestStatusTransitions(t *testing.T) {
	// Клиент может отменять только до начала ремонта.
	if !domain.StatusNew.CanBeCanceledByClient() {
		t.Error("статус new должен отменяться клиентом")
	}
	if !domain.StatusConfirmed.CanBeCanceledByClient() {
		t.Error("статус confirmed должен отменяться клиентом")
	}
	if domain.StatusInProgress.CanBeCanceledByClient() {
		t.Error("статус in_progress не должен отменяться клиентом")
	}
	if domain.StatusReady.CanBeCanceledByClient() {
		t.Error("статус ready не должен отменяться клиентом")
	}

	// Менеджер не может пропустить стадии.
	if containsStatus(domain.ManagerTransitions[domain.StatusNew], domain.StatusCompleted) {
		t.Error("new -> completed должен быть запрещён")
	}
	if !containsStatus(domain.ManagerTransitions[domain.StatusInProgress], domain.StatusReady) {
		t.Error("in_progress -> ready должен быть разрешён")
	}
	if !containsStatus(domain.ManagerTransitions[domain.StatusReady], domain.StatusCompleted) {
		t.Error("ready -> completed должен быть разрешён")
	}
}

// TestValidatePublicOrder проверяет валидацию экспресс-заявки.
func TestValidatePublicOrder(t *testing.T) {
	valid := PublicOrderRequest{
		Phone: "+79991234567", Password: "secret123", Name: "Иван",
		BranchID: 1, CarNumber: "А123ВС777", ServiceType: "Замена масла",
		PreferredDate: "2026-10-25", PreferredTime: "10:00",
	}
	if err := validatePublicOrder(valid); err != nil {
		t.Fatalf("валидная заявка отклонена: %v", err)
	}

	invalid := valid
	invalid.Phone = "123"
	invalid.Password = "123"
	invalid.CarNumber = ""
	err := validatePublicOrder(invalid)
	if err == nil {
		t.Fatal("ожидалась ошибка валидации")
	}
	verr, ok := err.(*ValidationError)
	if !ok {
		t.Fatalf("ожидалась *ValidationError, получено %T", err)
	}
	for _, field := range []string{"phone", "password", "car_number"} {
		if _, found := verr.Fields[field]; !found {
			t.Errorf("в ошибке отсутствует поле %q", field)
		}
	}
}

// TestParseClockToMinutes проверяет перевод времени в минуты.
func TestParseClockToMinutes(t *testing.T) {
	cases := []struct {
		in   string
		want int
	}{
		{"09:00", 540},
		{"21:00", 1260},
		{"12:30", 750},
	}
	for _, c := range cases {
		got, err := parseClockToMinutes(c.in)
		if err != nil {
			t.Fatalf("parseClockToMinutes(%q): %v", c.in, err)
		}
		if got != c.want {
			t.Errorf("parseClockToMinutes(%q) = %d, ожидалось %d", c.in, got, c.want)
		}
	}

	if _, err := parseClockToMinutes("bad"); err == nil {
		t.Error("ожидалась ошибка для некорректного времени")
	}
}
