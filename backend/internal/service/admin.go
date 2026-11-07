package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/sto/crm/backend/internal/domain"
	"github.com/sto/crm/backend/internal/store"
)

// AdminService — операции суперадминистратора.
type AdminService struct {
	branches *store.BranchRepo
	users    *store.UserRepo
	stats    *store.StatsRepo
	auth     *AuthService
	capacity int
}

// NewAdminService создаёт сервис администратора.
func NewAdminService(branches *store.BranchRepo, users *store.UserRepo, stats *store.StatsRepo, auth *AuthService, capacity int) *AdminService {
	return &AdminService{branches: branches, users: users, stats: stats, auth: auth, capacity: capacity}
}

// ListBranches возвращает список всех филиалов.
func (s *AdminService) ListBranches(ctx context.Context) ([]domain.Branch, error) {
	return s.branches.List(ctx)
}

// CreateBranch создаёт новый филиал СТО.
func (s *AdminService) CreateBranch(ctx context.Context, address string) (*domain.Branch, error) {
	address = strings.TrimSpace(address)
	if address == "" {
		return nil, newValidationError("Укажите адрес", map[string]string{"address": "обязательное поле"})
	}
	return s.branches.Create(ctx, address)
}

// CreateManagerRequest — данные для создания аккаунта менеджера.
type CreateManagerRequest struct {
	Phone    string `json:"phone"`
	Password string `json:"password"`
	Name     string `json:"name"`
	BranchID int    `json:"branch_id"`
}

// CreateManager создаёт менеджера и привязывает его к филиалу.
func (s *AdminService) CreateManager(ctx context.Context, req CreateManagerRequest) (*domain.User, error) {
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
	if req.BranchID <= 0 {
		fields["branch_id"] = "выберите филиал"
	}
	if len(fields) > 0 {
		return nil, newValidationError("Проверьте заполненные поля", fields)
	}

	if _, err := s.branches.Get(ctx, req.BranchID); err != nil {
		return nil, newValidationError("Филиал не найден", map[string]string{"branch_id": "неизвестный филиал"})
	}

	hash, err := hashPassword(req.Password)
	if err != nil {
		return nil, err
	}
	branchID := req.BranchID
	return s.users.Create(ctx, &domain.User{
		Phone:        normalizePhone(req.Phone),
		PasswordHash: hash,
		Name:         strings.TrimSpace(req.Name),
		Role:         domain.RoleManager,
		BranchID:     &branchID,
	})
}

// Dashboard возвращает сводную информацию по загруженности всех филиалов.
func (s *AdminService) Dashboard(ctx context.Context) (map[string]any, error) {
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	stats, err := s.stats.DayStatsForAllBranches(ctx, today)
	if err != nil {
		return nil, err
	}

	branches := make([]domain.BranchStat, 0, len(stats))
	totalToday, totalNew, totalCanceled := 0, 0, 0
	for _, st := range stats {
		pct := 0
		if s.capacity > 0 {
			pct = st.Total * 100 / s.capacity
		}
		bs := domain.BranchStat{
			BranchID:       st.BranchID,
			Address:        st.Address,
			Today:          st.Total,
			New:            st.ByStatus[string(domain.StatusNew)],
			InProgress:    st.ByStatus[string(domain.StatusInProgress)],
			Ready:          st.ByStatus[string(domain.StatusReady)],
			CanceledToday:  st.Canceled,
			LoadPct:       pct,
			LoadLevel:     LoadLevel(pct),
			ByStatus:       st.ByStatus,
		}
		branches = append(branches, bs)
		totalToday += bs.Today
		totalNew += bs.New
		totalCanceled += bs.CanceledToday
	}

	return map[string]any{
		"date":          today.Format("2006-01-02"),
		"branches":      branches,
		"total_today":   totalToday,
		"total_new":     totalNew,
		"total_canceled": totalCanceled,
	}, nil
}

// EnsureSuperadmin создаёт суперадмина, если в системе его ещё нет.
func EnsureSuperadmin(ctx context.Context, users *store.UserRepo, phone, password, name string) error {
	var count int
	if err := users.Pool().QueryRow(ctx,
		`SELECT COUNT(*) FROM users WHERE role = 'superadmin'`).Scan(&count); err != nil {
		return fmt.Errorf("не удалось проверить наличие суперадмина: %w", err)
	}
	if count > 0 {
		return nil
	}
	hash, err := hashPassword(password)
	if err != nil {
		return err
	}
	_, err = users.Create(ctx, &domain.User{
		Phone:        normalizePhone(phone),
		PasswordHash: hash,
		Name:         name,
		Role:         domain.RoleSuperadmin,
	})
	if err != nil && !strings.Contains(err.Error(), "конфликт") {
		return err
	}
	return nil
}
