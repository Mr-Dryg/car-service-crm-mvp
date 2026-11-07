// Package service содержит бизнес-логику CRM.
package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"

	"github.com/sto/crm/backend/internal/domain"
	"github.com/sto/crm/backend/internal/store"
)

// Ошибки бизнес-логики, транслируемые в HTTP-статусы.
var (
	ErrInvalidCredentials = errors.New("неверный телефон или пароль")
	ErrValidation         = errors.New("ошибка валидации")
	ErrForbidden          = errors.New("доступ запрещён")
	ErrConflict           = errors.New("конфликт состояния")
	ErrNotFound           = errors.New("объект не найден")
)

// ValidationError — ошибка валидации с деталями по полям.
type ValidationError struct {
	Message string
	Fields  map[string]string
}

func (e *ValidationError) Error() string { return e.Message }

func newValidationError(msg string, fields map[string]string) *ValidationError {
	return &ValidationError{Message: msg, Fields: fields}
}

// AuthService — регистрация, вход и работа с JWT-токенами.
type AuthService struct {
	users   *store.UserRepo
	secret  []byte
	ttl     time.Duration
}

// NewAuthService создаёт сервис аутентификации.
func NewAuthService(users *store.UserRepo, secret string, ttl time.Duration) *AuthService {
	return &AuthService{users: users, secret: []byte(secret), ttl: ttl}
}

// LoginResult — результат успешного входа.
type LoginResult struct {
	Token string     `json:"token"`
	User  domain.User `json:"user"`
}

// Login проверяет телефон и пароль, возвращая JWT и профиль пользователя.
func (s *AuthService) Login(ctx context.Context, phone, password string) (*LoginResult, error) {
	phone = normalizePhone(phone)
	if phone == "" || password == "" {
		return nil, &ValidationError{Message: "Телефон и пароль обязательны", Fields: map[string]string{
			"phone": "обязательное поле", "password": "обязательное поле",
		}}
	}

	user, err := s.users.GetByPhone(ctx, phone)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, ErrInvalidCredentials
		}
		return nil, err
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return nil, ErrInvalidCredentials
	}

	token, err := s.issueToken(user)
	if err != nil {
		return nil, err
	}
	return &LoginResult{Token: token, User: *user}, nil
}

// Register создаёт нового пользователя с захешированным паролем.
func (s *AuthService) Register(ctx context.Context, phone, password, name string, role domain.Role, branchID *int) (*domain.User, error) {
	phone = normalizePhone(phone)
	if err := validateRegistration(phone, password, name); err != nil {
		return nil, err
	}
	if !role.Valid() {
		role = domain.RoleClient
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("не удалось захешировать пароль: %w", err)
	}

	return s.users.Create(ctx, &domain.User{
		Phone:        phone,
		PasswordHash: string(hash),
		Name:         strings.TrimSpace(name),
		Role:         role,
		BranchID:     branchID,
	})
}

// issueToken выпускает подписанный JWT для пользователя.
func (s *AuthService) issueToken(u *domain.User) (string, error) {
	now := time.Now()
	claims := jwt.MapClaims{
		"sub":      u.ID,
		"phone":    u.Phone,
		"name":     u.Name,
		"role":     string(u.Role),
		"iat":      now.Unix(),
		"exp":      now.Add(s.ttl).Unix(),
		"issuer":   "sto-crm",
	}
	if u.BranchID != nil {
		claims["branch_id"] = *u.BranchID
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(s.secret)
}

// ParseToken валидирует JWT и возвращает содержимое токена.
func (s *AuthService) ParseToken(tokenStr string) (*Claims, error) {
	token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("неподдерживаемый алгоритм подписи")
		}
		return s.secret, nil
	})
	if err != nil {
		return nil, ErrInvalidCredentials
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || !token.Valid {
		return nil, ErrInvalidCredentials
	}

	sub, _ := claims["sub"].(float64)
	roleStr, _ := claims["role"].(string)
	out := &Claims{
		UserID: int(sub),
		Phone:  str(claims["phone"]),
		Name:   str(claims["name"]),
		Role:   domain.Role(roleStr),
	}
	if v, ok := claims["branch_id"].(float64); ok {
		out.BranchID = int(v)
	}
	return out, nil
}

// Claims — данные, извлечённые из JWT.
type Claims struct {
	UserID   int
	Phone    string
	Name     string
	Role     domain.Role
	BranchID int
}

// normalizePhone приводит телефон к каноничному виду: только цифры и ведущая «+».
func normalizePhone(phone string) string {
	phone = strings.TrimSpace(phone)
	if phone == "" {
		return ""
	}
	plus := strings.HasPrefix(phone, "+")
	digits := make([]rune, 0, len(phone))
	for _, r := range phone {
		if r >= '0' && r <= '9' {
			digits = append(digits, r)
		}
	}
	res := string(digits)
	if plus && res != "" {
		return "+" + res
	}
	return res
}

// validateRegistration проверяет обязательные поля регистрации.
func validateRegistration(phone, password, name string) error {
	fields := map[string]string{}
	if len(phone) < 10 || len(phone) > 20 {
		fields["phone"] = "телефон должен содержать 10–20 цифр"
	}
	if len(password) < 6 {
		fields["password"] = "пароль должен быть не короче 6 символов"
	}
	if strings.TrimSpace(name) == "" {
		fields["name"] = "имя обязательно"
	}
	if len(fields) > 0 {
		return newValidationError("Проверьте заполненные поля", fields)
	}
	return nil
}

// str безопасно приводит значение claim к строке.
func str(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}
