package service

import (
	"errors"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"

	"blacklist-index/internal/repository"
)

const (
	maxLoginAttempts = 5
	lockDuration     = 15 * time.Minute
)

var (
	// ErrTooManyAttempts 表示 IP 已被临时锁定。
	ErrTooManyAttempts = errors.New("尝试次数过多，请稍后再试")
	// ErrBadCredentials 表示用户名或动态口令错误。
	ErrBadCredentials = errors.New("用户名或动态口令错误")
)

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

// Login 验证密码与 TOTP，成功返回 JWT。
func (s *AuthService) Login(password, totpCode, ip, ua string) (string, error) {
	if s.isLocked(ip) {
		return "", ErrTooManyAttempts
	}

	admin, err := s.adminRepo.GetAdmin()
	if err != nil {
		return "", err
	}
	validPassword := bcrypt.CompareHashAndPassword([]byte(admin.PasswordHash), []byte(password)) == nil
	validTOTP := VerifyTOTP(admin.TOTPSecret, totpCode, time.Now())

	if !validPassword || !validTOTP {
		s.recordFailure(ip)
		return "", ErrBadCredentials
	}

	s.clearFailures(ip)
	token, err := GenerateJWT("admin", s.jwtSecret, 8*time.Hour)
	if err != nil {
		return "", err
	}
	s.audit.Log("admin", ActionLogin, "", ip, ua)
	return token, nil
}

// Logout 记录退出登录审计。
func (s *AuthService) Logout(ip, ua string) {
	s.audit.Log("admin", ActionLogout, "", ip, ua)
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
