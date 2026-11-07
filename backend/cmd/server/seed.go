package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/sto/crm/backend/internal/config"
	"github.com/sto/crm/backend/internal/domain"
	"github.com/sto/crm/backend/internal/service"
	"github.com/sto/crm/backend/internal/store"
)

// seed создаёт демонстрационные данные, если база пуста:
// два филиала, менеджера на каждый, клиента и несколько заявок.
func seed(
	ctx context.Context,
	branchRepo *store.BranchRepo,
	userRepo *store.UserRepo,
	orderRepo *store.OrderRepo,
	authSvc *service.AuthService,
	cfg *config.Config,
	logger *log.Logger,
) error {
	branches, err := branchRepo.List(ctx)
	if err != nil {
		return err
	}

	// Филиалы.
	if len(branches) == 0 {
		addresses := []string{
			"г. Москва, ул. Автозаводская, д. 12",
			"г. Москва, Ленинградский пр-т, д. 45",
		}
		for _, a := range addresses {
			if _, err := branchRepo.Create(ctx, a); err != nil {
				return fmt.Errorf("не удалось создать филиал: %w", err)
			}
		}
		if branches, err = branchRepo.List(ctx); err != nil {
			return err
		}
		logger.Printf("создано %d демо-филиала(ов)", len(branches))
	}

	// Менеджеры филиалов.
	for i, b := range branches {
		phone := fmt.Sprintf("+7900000100%d", i)
		if _, err := userRepo.GetByPhone(ctx, phone); err == nil {
			continue
		} else if !errors.Is(err, store.ErrNotFound) {
			return err
		}
		branchID := b.ID
		if _, err := userRepo.Create(ctx, &domain.User{
			Phone:        phone,
			PasswordHash: mustHash("manager123"),
			Name:         fmt.Sprintf("Менеджер %d", i+1),
			Role:         domain.RoleManager,
			BranchID:     &branchID,
		}); err != nil {
			return fmt.Errorf("не удалось создать менеджера: %w", err)
		}
		logger.Printf("создан менеджер %s (филиал %d)", phone, b.ID)
	}

	// Демо-клиент.
	clientPhone := "+79000002001"
	client, err := userRepo.GetByPhone(ctx, clientPhone)
	if err != nil {
		if !errors.Is(err, store.ErrNotFound) {
			return err
		}
		client, err = userRepo.Create(ctx, &domain.User{
			Phone:        clientPhone,
			PasswordHash: mustHash("client123"),
			Name:         "Иван Петров",
			Role:         domain.RoleClient,
		})
		if err != nil {
			return fmt.Errorf("не удалось создать демо-клиента: %w", err)
		}
		logger.Printf("создан демо-клиент %s", clientPhone)
	}

	// Демо-заявки на сегодня и завтра.
	var existing int
	if err := orderRepo.Pool().QueryRow(ctx, `SELECT COUNT(*) FROM orders`).Scan(&existing); err != nil {
		return err
	}
	if existing > 0 {
		return nil
	}

	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	tomorrow := today.AddDate(0, 0, 1)

	type demo struct {
		branch   int
		car      string
		service  string
		date     time.Time
		time     string
		comment  string
		status   domain.OrderStatus
		enRoute  bool
	}
	demos := []demo{
		{branches[0].ID, "А123ВС777", "Замена масла и фильтров", today, "10:00", "Двигатель 1.8", domain.StatusNew, false},
		{branches[0].ID, "В456ОР199", "Диагностика ходовой", today, "12:00", "Стук в подвеске", domain.StatusConfirmed, false},
		{branches[0].ID, "Е789МН750", "Замена тормозных колодок", today, "14:00", "", domain.StatusInProgress, false},
		{branches[0].ID, "К321АВ199", "Компьютерная диагностика", today, "16:00", "Горит чек", domain.StatusReady, false},
		{branches[0].ID, "Н654ТР750", "Замена ремня ГРМ", tomorrow, "09:30", "Ремонт по срочности", domain.StatusNew, false},
		{branches[0].ID, "О987АС199", "Плановое ТО", tomorrow, "11:30", "", domain.StatusNew, false},
		{branches[1].ID, "Р147КХ199", "Замена масла", today, "11:00", "", domain.StatusNew, false},
		{branches[1].ID, "Т258МУ750", "Ремонт кондиционера", today, "15:00", "Не дует", domain.StatusConfirmed, false},
	}

	for _, d := range demos {
		if _, err := orderRepo.Create(ctx, &domain.Order{
			ClientID:      client.ID,
			BranchID:      d.branch,
			CarNumber:     d.car,
			ServiceType:   d.service,
			PreferredDate: d.date.Format("2006-01-02"),
			PreferredTime: d.time,
			Comment:       d.comment,
			Status:        d.status,
			IsClientEnRoute: d.enRoute,
		}); err != nil {
			return fmt.Errorf("не удалось создать демо-заявку: %w", err)
		}
	}
	logger.Printf("создано %d демо-заявок(и)", len(demos))
	return nil
}

// mustHash хеширует пароль, паникуя при ошибке (используется только при seed).
func mustHash(password string) string {
	h, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		panic(err)
	}
	return string(h)
}
