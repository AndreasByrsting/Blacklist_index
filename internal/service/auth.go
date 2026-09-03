package service

import (
	"crypto/rand"
	"encoding/base32"
	"errors"
	"math/big"
	"regexp"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"

	"blacklist-index/internal/model"
	"blacklist-index/internal/repository"
)

const (
	maxLoginAttempts = 5
	lockDuration     = 15 * time.Minute
)

var (
	// ErrTooManyAttempts 表示 IP 已被临时锁定。
	ErrTooManyAttempts = errors.New("尝试次数过多，请稍后再试")
	// ErrBadCredentials 表示用户名或密码或动态口令错误。
	ErrBadCredentials = errors.New("用户名或密码或动态口令错误")
	// ErrWrongPassword 表示当前密码错误。
	ErrWrongPassword = errors.New("当前密码错误")
	// ErrWeakPassword 表示新密码强度不足。
	ErrWeakPassword = errors.New("新密码需至少 8 位，且同时包含大写字母、小写字母和数字")
	// ErrInvalidUsername 表示用户名不合法。
	ErrInvalidUsername = errors.New("用户名需 3-32 位，只能包含字母、数字、下划线与减号")
	// ErrCannotDeleteSelf 表示不能删除自己。
	ErrCannotDeleteSelf = errors.New("不能删除当前登录的账户")
	// ErrNeedAtLeastOneSuper 表示至少需要保留一名超级管理员。
	ErrNeedAtLeastOneSuper = errors.New("至少需要保留一名超级管理员")
	// ErrUsernameExists 表示用户名已存在。
	ErrUsernameExists = errors.New("用户名已存在")
)

var usernameRe = regexp.MustCompile(`^[a-zA-Z0-9_-]{3,32}$`)

type failState struct {
	count       int
	lockedUntil time.Time
}

// AuthService 负责管理员认证与登录限流。
type AuthService struct {
	adminRepo *repository.AdminRepo
	audit     *AuditService
	jwtSecret []byte
	loc       *time.Location

	mu       sync.Mutex
	failures map[string]*failState
}

func NewAuthService(adminRepo *repository.AdminRepo, audit *AuditService, jwtSecret []byte, loc *time.Location) *AuthService {
	return &AuthService{
		adminRepo: adminRepo,
		audit:     audit,
		jwtSecret: jwtSecret,
		loc:       loc,
		failures:  make(map[string]*failState),
	}
}

// Login 验证用户名+密码+TOTP，成功返回 JWT 与用户信息。
func (s *AuthService) Login(username, password, totpCode, ip, ua string) (string, *model.AdminUser, error) {
	if s.isLocked(ip) {
		return "", nil, ErrTooManyAttempts
	}

	admin, err := s.adminRepo.GetByUsername(username)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			s.recordFailure(ip)
			return "", nil, ErrBadCredentials
		}
		return "", nil, err
	}
	validPassword := bcrypt.CompareHashAndPassword([]byte(admin.PasswordHash), []byte(password)) == nil
	validTOTP := VerifyTOTP(admin.TOTPSecret, totpCode, time.Now())

	if !validPassword || !validTOTP {
		s.recordFailure(ip)
		return "", nil, ErrBadCredentials
	}

	s.clearFailures(ip)
	token, err := GenerateJWT(admin.Username, admin.IsSuper, s.jwtSecret, 8*time.Hour)
	if err != nil {
		return "", nil, err
	}
	s.audit.Log(admin.Username, ActionLogin, "", ip, ua)
	return token, admin, nil
}

// Logout 记录退出登录审计。
func (s *AuthService) Logout(username, ip, ua string) {
	s.audit.Log(username, ActionLogout, "", ip, ua)
}

// ChangePassword 校验当前密码后更新管理员自己的密码。
func (s *AuthService) ChangePassword(username, oldPassword, newPassword, ip, ua string) error {
	admin, err := s.adminRepo.GetByUsername(username)
	if err != nil {
		return err
	}
	if bcrypt.CompareHashAndPassword([]byte(admin.PasswordHash), []byte(oldPassword)) != nil {
		return ErrWrongPassword
	}
	if !strongPassword(newPassword) {
		return ErrWeakPassword
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), 12)
	if err != nil {
		return err
	}
	if err := s.adminRepo.UpdatePassword(admin.ID, string(hash)); err != nil {
		return err
	}
	s.audit.Log(username, ActionChangePassword, "", ip, ua)
	return nil
}

// ========== 超级管理员：用户管理 ==========

// CreateAdmin 由超级管理员创建新管理员。返回生成的随机密码与 TOTP 密钥。
func (s *AuthService) CreateAdmin(actorUsername, newUsername string, isSuper bool, ip, ua string) (string, string, error) {
	if !usernameRe.MatchString(newUsername) {
		return "", "", ErrInvalidUsername
	}
	password, err := GenerateStrongPassword()
	if err != nil {
		return "", "", err
	}
	totpSecret := GenerateTOTPSecret()
	hash, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		return "", "", err
	}
	a := &model.AdminUser{
		Username:     newUsername,
		PasswordHash: string(hash),
		TOTPSecret:   totpSecret,
		IsSuper:      isSuper,
		CreatedAt:    NowStr(s.loc),
		CreatedBy:    actorUsername,
	}
	if err := s.adminRepo.Create(a); err != nil {
		if errors.Is(err, repository.ErrUsernameExists) {
			return "", "", ErrUsernameExists
		}
		return "", "", err
	}
	s.audit.Log(actorUsername, ActionCreateAdmin, newUsername, ip, ua)
	return password, totpSecret, nil
}

// ResetPassword 由超级管理员重置指定管理员的密码，返回新随机密码（仅显示一次）。
func (s *AuthService) ResetPassword(actorUsername string, targetID int64, ip, ua string) (string, error) {
	target, err := s.adminRepo.GetByID(targetID)
	if err != nil {
		return "", err
	}
	password, err := GenerateStrongPassword()
	if err != nil {
		return "", err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		return "", err
	}
	if err := s.adminRepo.UpdatePassword(target.ID, string(hash)); err != nil {
		return "", err
	}
	s.audit.Log(actorUsername, ActionResetPassword, target.Username, ip, ua)
	return password, nil
}

// ResetTOTP 由超级管理员重置指定管理员的 TOTP 密钥，返回新密钥（仅显示一次）。
func (s *AuthService) ResetTOTP(actorUsername string, targetID int64, ip, ua string) (string, error) {
	target, err := s.adminRepo.GetByID(targetID)
	if err != nil {
		return "", err
	}
	secret := GenerateTOTPSecret()
	if err := s.adminRepo.UpdateTOTPSecret(target.ID, secret); err != nil {
		return "", err
	}
	s.audit.Log(actorUsername, ActionResetTOTP, target.Username, ip, ua)
	return secret, nil
}

// DeleteAdmin 由超级管理员删除指定管理员。禁止删除自己，且至少保留一名超级管理员。
func (s *AuthService) DeleteAdmin(actorUsername string, targetID int64, ip, ua string) error {
	target, err := s.adminRepo.GetByID(targetID)
	if err != nil {
		return err
	}
	if target.Username == actorUsername {
		return ErrCannotDeleteSelf
	}
	// 若是超级管理员，需保证至少还有一名超级管理员
	if target.IsSuper {
		list, err := s.adminRepo.List()
		if err != nil {
			return err
		}
		superCount := 0
		for _, a := range list {
			if a.IsSuper && a.ID != targetID {
				superCount++
			}
		}
		if superCount == 0 {
			return ErrNeedAtLeastOneSuper
		}
	}
	if err := s.adminRepo.Delete(targetID); err != nil {
		return err
	}
	s.audit.Log(actorUsername, ActionDeleteAdmin, target.Username, ip, ua)
	return nil
}

// ListAdmins 列出所有普通管理员（不含超级管理员与敏感字段）。
func (s *AuthService) ListAdmins() ([]*model.AdminUser, error) {
	all, err := s.adminRepo.List()
	if err != nil {
		return nil, err
	}
	list := make([]*model.AdminUser, 0, len(all))
	for _, a := range all {
		if !a.IsSuper {
			list = append(list, a)
		}
	}
	return list, nil
}

// GetAdminByUsername 按用户名获取管理员信息。
func (s *AuthService) GetAdminByUsername(username string) (*model.AdminUser, error) {
	return s.adminRepo.GetByUsername(username)
}

// GetAdminByID 按 ID 获取管理员信息。
func (s *AuthService) GetAdminByID(id int64) (*model.AdminUser, error) {
	return s.adminRepo.GetByID(id)
}

// ========== 工具函数 ==========

const strongPasswordLength = 8

// strongPassword 校验密码强度：≥8 位且包含大小写字母与数字。
func strongPassword(p string) bool {
	if len(p) < strongPasswordLength {
		return false
	}
	var hasLower, hasUpper, hasDigit bool
	for _, r := range p {
		switch {
		case r >= 'a' && r <= 'z':
			hasLower = true
		case r >= 'A' && r <= 'Z':
			hasUpper = true
		case r >= '0' && r <= '9':
			hasDigit = true
		}
	}
	return hasLower && hasUpper && hasDigit
}

const (
	passwordLower   = "abcdefghijklmnopqrstuvwxyz"
	passwordUpper   = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	passwordDigits  = "0123456789"
	passwordSymbols = "!@#$%^&*()-_=+[]{};:,.<>?"
)

// GenerateStrongPassword 生成 8 位强随机密码：数字+大小写字母+半角符号，每类至少一位。
func GenerateStrongPassword() (string, error) {
	const n = 8
	charsets := []string{passwordLower, passwordUpper, passwordDigits, passwordSymbols}
	result := make([]byte, n)
	// 先确保每类至少一位
	for i, cs := range charsets {
		idx, err := randInt(len(cs))
		if err != nil {
			return "", err
		}
		result[i] = cs[idx]
	}
	// 剩余位从全字符集中随机
	all := passwordLower + passwordUpper + passwordDigits + passwordSymbols
	for i := len(charsets); i < n; i++ {
		idx, err := randInt(len(all))
		if err != nil {
			return "", err
		}
		result[i] = all[idx]
	}
	// Fisher–Yates 打乱
	for i := n - 1; i > 0; i-- {
		j, err := randInt(i + 1)
		if err != nil {
			return "", err
		}
		result[i], result[j] = result[j], result[i]
	}
	return string(result), nil
}

// GenerateTOTPSecret 生成 20 字符 Base32 TOTP 密钥（100 位熵）。
func GenerateTOTPSecret() string {
	b := make([]byte, 20)
	_, _ = rand.Read(b)
	return base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(b)
}

func randInt(max int) (int, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(int64(max)))
	if err != nil {
		return 0, err
	}
	return int(n.Int64()), nil
}

func (s *AuthService) isLocked(ip string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	fs := s.failures[ip]
	if fs == nil {
		return false
	}
	return time.Now().Before(fs.lockedUntil)
}

func (s *AuthService) recordFailure(ip string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	fs := s.failures[ip]
	if fs == nil {
		fs = &failState{}
		s.failures[ip] = fs
	}
	fs.count++
	if s.failures[ip].count >= maxLoginAttempts {
		s.failures[ip].lockedUntil = time.Now().Add(lockDuration)
		s.failures[ip].count = 0
	}
}

func (s *AuthService) clearFailures(ip string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.failures, ip)
}
