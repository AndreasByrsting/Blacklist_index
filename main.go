package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"golang.org/x/crypto/bcrypt"

	"blacklist-index/internal/config"
	"blacklist-index/internal/db"
	"blacklist-index/internal/handler"
	"blacklist-index/internal/logger"
	"blacklist-index/internal/model"
	"blacklist-index/internal/repository"
	"blacklist-index/internal/server"
	"blacklist-index/internal/service"
)

const alphaNum = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "配置错误: %v\n", err)
		os.Exit(1)
	}

	// 初始化日志（JSON 输出 + 按日期滚动文件）
	dw, err := logger.NewDailyWriter(filepath.Join(cfg.DataDir, "logs"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "日志初始化失败: %v\n", err)
		os.Exit(1)
	}
	defer dw.Close()
	slog.SetDefault(slog.New(slog.NewJSONHandler(io.MultiWriter(os.Stdout, dw), nil)))

	// 打开数据库
	database, err := db.Open(cfg.DataDir)
	if err != nil {
		slog.Error("数据库打开失败", "err", err)
		os.Exit(1)
	}
	defer database.Close()

	if err := db.IntegrityCheck(database); err != nil {
		slog.Error("数据库完整性检查失败", "err", err)
		os.Exit(1)
	}
	if err := db.Migrate(database); err != nil {
		slog.Error("数据库迁移失败", "err", err)
		os.Exit(1)
	}

	// 仓储
	configRepo := repository.NewConfigRepo(database)
	adminRepo := repository.NewAdminRepo(database)
	blacklistRepo := repository.NewBlacklistRepo(database)
	announcementRepo := repository.NewAnnouncementRepo(database)
	auditRepo := repository.NewAuditRepo(database)

	// 初始化动态配置
	dashboardPath, err := getOrCreate(configRepo, "dashboard_path", func() string { return randAlpha(8) })
	if err != nil {
		slog.Error("初始化 dashboard_path 失败", "err", err)
		os.Exit(1)
	}
	jwtSecretHex, err := getOrCreate(configRepo, "jwt_secret", func() string {
		b := make([]byte, 32)
		if _, err := rand.Read(b); err != nil {
			panic(err)
		}
		return hex.EncodeToString(b)
	})
	if err != nil {
		slog.Error("初始化 jwt_secret 失败", "err", err)
		os.Exit(1)
	}
	jwtSecret, err := hex.DecodeString(jwtSecretHex)
	if err != nil {
		slog.Error("jwt_secret 解码失败", "err", err)
		os.Exit(1)
	}

	// 初始化管理员
	adminCreated, err := initAdmin(adminRepo, cfg)
	if err != nil {
		slog.Error("初始化管理员失败", "err", err)
		os.Exit(1)
	}

	// 服务
	auditSvc := service.NewAuditService(auditRepo, cfg.Location)
	authSvc := service.NewAuthService(adminRepo, auditSvc, jwtSecret, cfg.Location)
	blacklistSvc := service.NewBlacklistService(blacklistRepo, auditSvc, cfg.Location)
	announcementSvc := service.NewAnnouncementService(announcementRepo, cfg.Location)
	settingSvc := service.NewSettingService(configRepo, auditSvc)
	submissionRepo := repository.NewSubmissionRepo(database)
	imageSvc := service.NewImageService(cfg.DataDir)
	submissionSvc := service.NewSubmissionService(submissionRepo, blacklistSvc, auditSvc, imageSvc, cfg.Location)

	// Handler 与路由
	h := handler.New(cfg, database, blacklistSvc, authSvc, announcementSvc, auditSvc, settingSvc, submissionSvc, imageSvc, jwtSecret)
	assets, err := fs.Sub(webFS, "web")
	if err != nil {
		slog.Error("加载静态资源失败", "err", err)
		os.Exit(1)
	}
	httpHandler := server.New(h, dashboardPath, cfg.SiteName, assets)

	if adminCreated {
		setupURL := service.TOTPSetupURL(cfg.TOTPSecret, cfg.SiteName, "admin")
		slog.Info("管理员账号已创建，请使用以下 URL 将 TOTP 绑定到验证器", "setup_url", setupURL)
	}

	slog.Info("服务启动",
		"site", cfg.SiteName,
		"port", cfg.Port,
		"dashboard", "/"+dashboardPath,
		"timezone", cfg.Timezone,
		"data_dir", cfg.DataDir,
	)

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           httpHandler,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("服务异常退出", "err", err)
			os.Exit(1)
		}
	}()

	// 优雅关机
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	slog.Info("收到退出信号，开始优雅关机")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		slog.Error("优雅关机失败", "err", err)
	}
}

// getOrCreate 读取配置项，不存在则生成并写入。
func getOrCreate(repo *repository.ConfigRepo, key string, gen func() string) (string, error) {
	v, err := repo.Get(key)
	if err == nil {
		return v, nil
	}
	if !errors.Is(err, repository.ErrNotFound) {
		return "", err
	}
	v = gen()
	if err := repo.Set(key, v); err != nil {
		return "", err
	}
	return v, nil
}

// initAdmin 若管理员不存在则用环境变量创建超级管理员，返回是否新建。
func initAdmin(repo *repository.AdminRepo, cfg *config.Config) (bool, error) {
	count, err := repo.Count()
	if err != nil {
		return false, err
	}
	if count > 0 {
		return false, nil
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(cfg.AdminPassword), 12)
	if err != nil {
		return false, err
	}
	a := &model.AdminUser{
		Username:     "admin",
		PasswordHash: string(hash),
		TOTPSecret:   cfg.TOTPSecret,
		IsSuper:      true,
		CreatedAt:    service.NowStr(cfg.Location),
		CreatedBy:    "system",
	}
	if err := repo.Create(a); err != nil {
		return false, err
	}
	return true, nil
}

// randAlpha 生成 n 位随机字母数字串。
func randAlpha(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	for i := range b {
		b[i] = alphaNum[int(b[i])%len(alphaNum)]
	}
	return string(b)
}
