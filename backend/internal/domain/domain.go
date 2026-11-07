// Package domain содержит доменные сущности CRM автосервиса.
package domain

import "time"

// Role — роль пользователя в системе.
type Role string

const (
	RoleClient     Role = "client"
	RoleManager    Role = "manager"
	RoleSuperadmin Role = "superadmin"
)

// Valid проверяет, что роль известна системе.
func (r Role) Valid() bool {
	switch r {
	case RoleClient, RoleManager, RoleSuperadmin:
		return true
	}
	return false
}

// OrderStatus — жизненный цикл заказа на ремонт.
type OrderStatus string

const (
	StatusNew        OrderStatus = "new"
	StatusConfirmed  OrderStatus = "confirmed"
	StatusCanceled   OrderStatus = "canceled"
	StatusInProgress OrderStatus = "in_progress"
	StatusReady      OrderStatus = "ready"
	StatusCompleted  OrderStatus = "completed"
)

// AllStatuses — полный набор статусов в порядке жизненного цикла.
var AllStatuses = []OrderStatus{
	StatusNew, StatusConfirmed, StatusInProgress, StatusReady, StatusCompleted, StatusCanceled,
}

// Valid проверяет, что статус известен системе.
func (s OrderStatus) Valid() bool {
	for _, st := range AllStatuses {
		if st == s {
			return true
		}
	}
	return false
}

// Label — человекочитаемое название статуса на русском.
func (s OrderStatus) Label() string {
	switch s {
	case StatusNew:
		return "Новая"
	case StatusConfirmed:
		return "Подтверждена"
	case StatusInProgress:
		return "В работе"
	case StatusReady:
		return "Готова"
	case StatusCompleted:
		return "Завершена"
	case StatusCanceled:
		return "Отменена"
	}
	return string(s)
}

// CanBeCanceledByClient — клиент может отменить заказ, пока ремонт не начался.
func (s OrderStatus) CanBeCanceledByClient() bool {
	return s == StatusNew || s == StatusConfirmed
}

// ManagerTransitions — допустимые переходы статуса, инициируемые менеджером.
// Клиентская отмена обрабатывается отдельно (см. CanBeCanceledByClient).
var ManagerTransitions = map[OrderStatus][]OrderStatus{
	StatusNew:        {StatusConfirmed, StatusInProgress, StatusCanceled},
	StatusConfirmed:  {StatusInProgress, StatusCanceled},
	StatusInProgress: {StatusReady, StatusCanceled},
	StatusReady:      {StatusCompleted, StatusInProgress},
	StatusCompleted:  {},
	StatusCanceled:   {StatusNew},
}

// Branch — филиал автосервиса.
type Branch struct {
	ID        int       `json:"id"`
	Address   string    `json:"address"`
	CreatedAt time.Time `json:"created_at"`
}

// User — пользователь системы (клиент, менеджер или суперадмин).
type User struct {
	ID           int       `json:"id"`
	Phone        string    `json:"phone"`
	PasswordHash string    `json:"-"`
	Name         string    `json:"name"`
	Role         Role      `json:"role"`
	BranchID     *int      `json:"branch_id,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
}

// Order — заявка на ремонт.
type Order struct {
	ID              int         `json:"id"`
	ClientID        int         `json:"client_id"`
	BranchID        int         `json:"branch_id"`
	CarNumber       string      `json:"car_number"`
	ServiceType     string      `json:"service_type"`
	PreferredDate   string      `json:"preferred_date"` // YYYY-MM-DD
	PreferredTime   string      `json:"preferred_time"` // HH:MM
	Comment         string      `json:"comment"`
	Status          OrderStatus `json:"status"`
	IsClientEnRoute bool        `json:"is_client_en_route"`
	CreatedAt       time.Time   `json:"created_at"`
	UpdatedAt       time.Time   `json:"updated_at"`

	// Денормализованные поля для удобства отображения в интерфейсах.
	ClientName    string `json:"client_name,omitempty"`
	ClientPhone   string `json:"client_phone,omitempty"`
	BranchAddress string `json:"branch_address,omitempty"`
}

// Slot — слот загрузки филиала на конкретную дату.
type Slot struct {
	Time       string `json:"time"`
	Orders     int    `json:"orders"`
	Capacity   int    `json:"capacity"`
	LoadPct    int    `json:"load_pct"`
	LoadLevel  string `json:"load_level"` // green | yellow | red
}

// DaySlots — агрегированная загруженность филиала за день.
type DaySlots struct {
	Date       string `json:"date"`
	BranchID   int    `json:"branch_id"`
	Total      int    `json:"total"`
	Capacity   int    `json:"capacity"`
	LoadPct    int    `json:"load_pct"`
	LoadLevel  string `json:"load_level"`
	Slots      []Slot `json:"slots"`
}

// BranchStat — сводка по филиалу для дашборда суперадмина.
type BranchStat struct {
	BranchID      int            `json:"branch_id"`
	Address       string         `json:"address"`
	Today         int            `json:"today"`
	New           int            `json:"new"`
	InProgress   int            `json:"in_progress"`
	Ready         int            `json:"ready"`
	CanceledToday int            `json:"canceled_today"`
	LoadPct      int            `json:"load_pct"`
	LoadLevel    string         `json:"load_level"`
	ByStatus      map[string]int `json:"by_status"`
}
