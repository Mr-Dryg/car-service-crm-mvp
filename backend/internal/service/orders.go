package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/sto/crm/backend/internal/domain"
	"github.com/sto/crm/backend/internal/store"
)

// Notifier — интерфейс отправки уведомлений клиенту.
// Реализация — заглушка с логированием в консоль, готовая к замене на реальный SMS-шлюз.
type Notifier interface {
	SendSMS(phone, message string)
	SendEmail(email, subject, body string)
}

// OrderService — бизнес-логика работы с заказами.
type OrderService struct {
	orders    *store.OrderRepo
	branches  *store.BranchRepo
	users     *store.UserRepo
	auth      *AuthService
	notifier  Notifier
	slots     SlotProvider
	capacity  int
	workStart string
	workEnd   string
}

// SlotProvider — источник данных по загруженности филиала для календаря.
type SlotProvider interface {
	DaySlots(ctx context.Context, branchID int, date time.Time) (*domain.DaySlots, error)
}

// NewOrderService создаёт сервис заказов.
func NewOrderService(
	orders *store.OrderRepo,
	branches *store.BranchRepo,
	users *store.UserRepo,
	auth *AuthService,
	notifier Notifier,
	slots SlotProvider,
	capacity int,
	workStart, workEnd string,
) *OrderService {
	return &OrderService{
		orders: orders, branches: branches, users: users, auth: auth,
		notifier: notifier, slots: slots, capacity: capacity,
		workStart: workStart, workEnd: workEnd,
	}
}

// PublicOrderRequest — заявка с сайта: одновременно регистрирует клиента и создаёт заказ.
type PublicOrderRequest struct {
	Phone         string `json:"phone"`
	Password      string `json:"password"`
	Name          string `json:"name"`
	BranchID      int    `json:"branch_id"`
	CarNumber     string `json:"car_number"`
	ServiceType   string `json:"service_type"`
	PreferredDate string `json:"preferred_date"`
	PreferredTime string `json:"preferred_time"`
	Comment       string `json:"comment"`
}

// PublicOrderResult — результат оформления заявки с сайта.
type PublicOrderResult struct {
	Token string       `json:"token"`
	User  domain.User  `json:"user"`
	Order domain.Order `json:"order"`
}

// CreatePublicOrder реализует сценарий «экспресс-запись и бесшовная регистрация»:
// по одному обращению создаются аккаунт клиента и заявка, клиент сразу авторизован.
func (s *OrderService) CreatePublicOrder(ctx context.Context, req PublicOrderRequest) (*PublicOrderResult, error) {
	if err := validatePublicOrder(req); err != nil {
		return nil, err
	}

	branch, err := s.branches.Get(ctx, req.BranchID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, newValidationError("Филиал не найден", map[string]string{"branch_id": "неизвестный филиал"})
		}
		return nil, err
	}

	phone := normalizePhone(req.Phone)
	date, clock, err := parseDateTime(req.PreferredDate, req.PreferredTime)
	if err != nil {
		return nil, err
	}

	// Если клиент с таким телефоном уже зарегистрирован — вход без смены пароля.
	client, err := s.users.GetByPhone(ctx, phone)
	switch {
	case err == nil:
		// Аккаунт существует: проверяем пароль, чтобы не дать создать заявку от чужого телефона.
		if err := comparePassword(client.PasswordHash, req.Password); err != nil {
			return nil, ErrInvalidCredentials
		}
	case errors.Is(err, store.ErrNotFound):
		hash, hashErr := hashPassword(req.Password)
		if hashErr != nil {
			return nil, hashErr
		}
		client, err = s.users.Create(ctx, &domain.User{
			Phone:        phone,
			PasswordHash: hash,
			Name:         strings.TrimSpace(req.Name),
			Role:         domain.RoleClient,
		})
		if err != nil {
			if errors.Is(err, store.ErrConflict) {
				return nil, newValidationError("Этот телефон уже зарегистрирован", map[string]string{
					"phone": "телефон уже используется — войдите в личный кабинет",
				})
			}
			return nil, err
		}
	default:
		return nil, err
	}

	order, err := s.orders.Create(ctx, &domain.Order{
		ClientID:      client.ID,
		BranchID:      branch.ID,
		CarNumber:     strings.TrimSpace(req.CarNumber),
		ServiceType:   strings.TrimSpace(req.ServiceType),
		PreferredDate: date,
		PreferredTime: clock,
		Comment:       strings.TrimSpace(req.Comment),
		Status:        domain.StatusNew,
	})
	if err != nil {
		return nil, err
	}

	token, err := s.issueClientToken(client)
	if err != nil {
		return nil, err
	}

	s.notifier.SendSMS(client.Phone, fmt.Sprintf(
		"СТО: заявка №%d на %s %s создана. Войдите в личный кабинет для отслеживания.",
		order.ID, date, clock))

	return &PublicOrderResult{Token: token, User: *client, Order: *order}, nil
}

// ListClientOrders возвращает заказы авторизованного клиента.
func (s *OrderService) ListClientOrders(ctx context.Context, clientID int) ([]domain.Order, error) {
	return s.orders.ListByClient(ctx, clientID)
}

// CancelOrder переводит заказ клиента в canceled.
// Отмена разрешена только пока ремонт не начался (статусы new и confirmed).
func (s *OrderService) CancelOrder(ctx context.Context, orderID, clientID int) (*domain.Order, error) {
	order, err := s.orders.Get(ctx, orderID)
	if err != nil {
		return nil, err
	}
	if order.ClientID != clientID {
		return nil, ErrForbidden
	}
	if !order.Status.CanBeCanceledByClient() {
		return nil, &ValidationError{
			Message: "Этот заказ уже нельзя отменить",
			Fields:  map[string]string{"status": "ремонт уже в работе"},
		}
	}

	updated, err := s.orders.UpdateStatus(ctx, orderID, domain.StatusCanceled)
	if err != nil {
		return nil, err
	}
	s.notifier.SendSMS(order.ClientPhone, fmt.Sprintf("СТО: заявка №%d отменена. Менеджер свяжется с вами.", orderID))
	return updated, nil
}

// SetEnRoute обрабатывает кнопку «Подтверждаю, скоро буду, не звоните».
// Разрешено только для заказов в статусе ready.
func (s *OrderService) SetEnRoute(ctx context.Context, orderID, clientID int) (*domain.Order, error) {
	order, err := s.orders.Get(ctx, orderID)
	if err != nil {
		return nil, err
	}
	if order.ClientID != clientID {
		return nil, ErrForbidden
	}
	if order.Status != domain.StatusReady {
		return nil, &ValidationError{
			Message: "Кнопка доступна только когда ремонт готов",
			Fields:  map[string]string{"status": "текущий статус: " + order.Status.Label()},
		}
	}

	updated, err := s.orders.SetEnRoute(ctx, orderID, true)
	if err != nil {
		return nil, err
	}
	s.notifier.SendSMS(order.ClientPhone, fmt.Sprintf(
		"СТО: спасибо! Ваша машина готова. Менеджер уведомлён, звонить не будем (заявка №%d).", orderID))
	return updated, nil
}

// ManagerListFilter — фильтры списка заказов филиала.
type ManagerListFilter struct {
	Status      string
	Date        string
	Search      string
	OnlyCanceled bool
}

// ListBranchOrders возвращает заказы филиала менеджера с фильтрами.
func (s *OrderService) ListBranchOrders(ctx context.Context, branchID int, f ManagerListFilter) ([]domain.Order, error) {
	filter := store.OrderFilter{Search: f.Search, OnlyCanceled: f.OnlyCanceled}
	if f.Status != "" {
		st := domain.OrderStatus(f.Status)
		if !st.Valid() {
			return nil, newValidationError("Неизвестный статус", map[string]string{"status": "некорректное значение"})
		}
		filter.Status = st
	}
	if f.Date != "" {
		d, err := time.Parse("2006-01-02", f.Date)
		if err != nil {
			return nil, newValidationError("Некорректная дата", map[string]string{"date": "ожидается YYYY-MM-DD"})
		}
		filter.Date = &d
	}
	return s.orders.ListByBranch(ctx, branchID, filter)
}

// ListToday возвращает план на сегодня для филиала.
func (s *OrderService) ListToday(ctx context.Context, branchID int) ([]domain.Order, error) {
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	return s.orders.ListByBranchAndDate(ctx, branchID, today)
}

// GetOrder возвращает заказ, проверяя принадлежность к филиалу менеджера.
func (s *OrderService) GetOrder(ctx context.Context, orderID, branchID int) (*domain.Order, error) {
	order, err := s.orders.Get(ctx, orderID)
	if err != nil {
		return nil, err
	}
	if order.BranchID != branchID {
		return nil, ErrForbidden
	}
	return order, nil
}

// ChangeStatus меняет статус заказа менеджером с проверкой допустимого перехода.
// При переходе в ready отправляется SMS клиенту.
func (s *OrderService) ChangeStatus(ctx context.Context, orderID, branchID int, newStatus domain.OrderStatus) (*domain.Order, error) {
	if !newStatus.Valid() {
		return nil, newValidationError("Неизвестный статус", map[string]string{"status": "некорректное значение"})
	}

	order, err := s.orders.Get(ctx, orderID)
	if err != nil {
		return nil, err
	}
	if order.BranchID != branchID {
		return nil, ErrForbidden
	}

	allowed := domain.ManagerTransitions[order.Status]
	if !containsStatus(allowed, newStatus) {
		return nil, &ValidationError{
			Message: fmt.Sprintf("Недопустимый переход: %s → %s", order.Status.Label(), newStatus.Label()),
			Fields:  map[string]string{"status": "недопустимый переход"},
		}
	}

	updated, err := s.orders.UpdateStatus(ctx, orderID, newStatus)
	if err != nil {
		return nil, err
	}

	if newStatus == domain.StatusReady {
		s.notifier.SendSMS(order.ClientPhone, fmt.Sprintf(
			"СТО: ваш автомобиль %s готов к выдаче! Войдите в личный кабинет и нажмите «Подтверждаю, скоро буду». Заявка №%d.",
			order.CarNumber, orderID))
	}
	return updated, nil
}

// RestoreOrder восстанавливает отменённый заказ с возможностью смены даты и времени.
func (s *OrderService) RestoreOrder(ctx context.Context, orderID, branchID int, dateStr, timeStr string) (*domain.Order, error) {
	order, err := s.orders.Get(ctx, orderID)
	if err != nil {
		return nil, err
	}
	if order.BranchID != branchID {
		return nil, ErrForbidden
	}
	if order.Status != domain.StatusCanceled {
		return nil, &ValidationError{
			Message: "Восстановить можно только отменённый заказ",
			Fields:  map[string]string{"status": "заказ не отменён"},
		}
	}

	date := order.PreferredDate
	clock := order.PreferredTime
	if dateStr != "" || timeStr != "" {
		d, c, err := parseDateTime(dateStr, timeStr)
		if err != nil {
			return nil, err
		}
		if dateStr != "" {
			date = d
		}
		if timeStr != "" {
			clock = c
		}
	}
	if _, err := s.orders.UpdateStatus(ctx, orderID, domain.StatusNew); err != nil {
		return nil, err
	}
	return s.orders.UpdateSchedule(ctx, orderID, date, clock)
}

// DeleteOrder окончательно удаляет отменённый заказ, освобождая слот.
func (s *OrderService) DeleteOrder(ctx context.Context, orderID, branchID int) error {
	order, err := s.orders.Get(ctx, orderID)
	if err != nil {
		return err
	}
	if order.BranchID != branchID {
		return ErrForbidden
	}
	if order.Status != domain.StatusCanceled {
		return &ValidationError{
			Message: "Удалить можно только отменённый клиентом заказ",
			Fields:  map[string]string{"status": "заказ не отменён"},
		}
	}
	return s.orders.Delete(ctx, orderID)
}

// GetSlots возвращает данные интеллектуального календаря на дату.
func (s *OrderService) GetSlots(ctx context.Context, branchID int, dateStr string) (*domain.DaySlots, error) {
	date, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		return nil, newValidationError("Некорректная дата", map[string]string{"date": "ожидается YYYY-MM-DD"})
	}
	return s.slots.DaySlots(ctx, branchID, date)
}

// issueClientToken выпускает JWT для клиента.
func (s *OrderService) issueClientToken(u *domain.User) (string, error) {
	return s.auth.issueToken(u)
}

// validatePublicOrder проверяет поля экспресс-заявки.
func validatePublicOrder(req PublicOrderRequest) error {
	fields := map[string]string{}
	if len(normalizePhone(req.Phone)) < 10 {
		fields["phone"] = "телефон должен содержать 10–20 цифр"
	}
	if len(req.Password) < 6 {
		fields["password"] = "пароль должен быть не короче 6 символов"
	}
	if strings.TrimSpace(req.Name) == "" {
		fields["name"] = "имя обязательно"
	}
	if strings.TrimSpace(req.CarNumber) == "" {
		fields["car_number"] = "укажите госномер"
	}
	if strings.TrimSpace(req.ServiceType) == "" {
		fields["service_type"] = "выберите тип обслуживания"
	}
	if req.BranchID <= 0 {
		fields["branch_id"] = "выберите филиал"
	}
	if strings.TrimSpace(req.PreferredDate) == "" {
		fields["preferred_date"] = "укажите дату"
	}
	if strings.TrimSpace(req.PreferredTime) == "" {
		fields["preferred_time"] = "укажите время"
	}
	if len(fields) > 0 {
		return newValidationError("Проверьте заполненные поля", fields)
	}
	return nil
}

// parseDateTime разбирает дату и время записи, возвращая их в каноничном виде.
func parseDateTime(dateStr, timeStr string) (string, string, error) {
	d, err := time.Parse("2006-01-02", strings.TrimSpace(dateStr))
	if err != nil {
		return "", "", newValidationError("Некорректная дата", map[string]string{
			"preferred_date": "ожидается YYYY-MM-DD",
		})
	}
	clock := strings.TrimSpace(timeStr)
	if _, err := time.Parse("15:04", clock); err != nil {
		if _, err2 := time.Parse("15:04:05", clock); err2 != nil {
			return "", "", newValidationError("Некорректное время", map[string]string{
				"preferred_time": "ожидается HH:MM",
			})
		}
		clock = d2Clock(clock)
	}
	return d.Format("2006-01-02"), clock, nil
}

// d2Clock нормализует время вида HH:MM:SS к HH:MM.
func d2Clock(clock string) string {
	if t, err := time.Parse("15:04:05", clock); err == nil {
		return t.Format("15:04")
	}
	return clock
}

// containsStatus проверяет наличие статуса в списке.
func containsStatus(list []domain.OrderStatus, s domain.OrderStatus) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}
