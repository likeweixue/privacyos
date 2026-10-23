// Command privacyos-server runs the PrivacyOS NAS management API and web panel.
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"privacyos/backend/internal/api"
	"privacyos/backend/internal/config"
	"privacyos/backend/internal/core"
	"privacyos/backend/internal/store"
	"privacyos/backend/internal/system"
	"privacyos/backend/internal/version"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lmsgprefix)
	log.SetPrefix("[privacyos] ")

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}
	st, err := store.Open(cfg)
	if err != nil {
		log.Fatalf("打开数据库失败: %v", err)
	}
	defer st.Close()

	app, err := core.New(cfg, st)
	if err != nil {
		log.Fatalf("初始化失败: %v", err)
	}
	app.StartupChecks()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go app.CleanupLoop(ctx)

	sys := system.New(cfg, st)
	go ddnsLoop(ctx, app, sys)

	apiServer := api.New(cfg, app)
	httpServer := &http.Server{
		Addr:              cfg.Listen,
		Handler:           apiServer.Handler(),
		ReadHeaderTimeout: 15 * time.Second,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}

	printBanner(cfg)

	errCh := make(chan error, 1)
	go func() {
		if cfg.TLSCert != "" {
			log.Printf("HTTPS 已启用，证书: %s", cfg.TLSCert)
			errCh <- httpServer.ListenAndServeTLS(cfg.TLSCert, cfg.TLSKey)
			return
		}
		errCh <- httpServer.ListenAndServe()
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-errCh:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("HTTP 服务异常退出: %v", err)
		}
	case sig := <-stop:
		log.Printf("收到信号 %s，正在关闭...", sig)
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 12*time.Second)
		defer shutdownCancel()
		_ = httpServer.Shutdown(shutdownCtx)
	}
	cancel()
	log.Printf("PrivacyOS 已停止")
}

// ddnsLoop refreshes dynamic DNS records when enabled.
func ddnsLoop(ctx context.Context, app *core.App, sys *system.Service) {
	ticker := time.NewTicker(2 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			settings := app.LoadSettings()
			ddns := settings.DDNS
			if !ddns.Enabled || ddns.Domain == "" {
				continue
			}
			interval := ddns.Interval
			if interval < 60 {
				interval = 300
			}
			if ddns.LastRun != "" {
				if last := store.Time(ddns.LastRun); time.Since(last) < time.Duration(interval)*time.Second {
					continue
				}
			}
			ip, err := sys.UpdateDDNS(ctx, ddns.Provider, ddns.Domain, ddns.Token, ddns.Username, ddns.Password, ddns.URL)
			ddns.LastRun = store.Now()
			if err != nil {
				ddns.LastErr = err.Error()
				log.Printf("DDNS 更新失败: %v", err)
			} else {
				ddns.LastErr = ""
				ddns.LastIP = ip
			}
			_ = app.Store.SetSetting("ddns", ddns)
		}
	}
}

// printBanner writes the startup banner to stdout (and the boot console).
func printBanner(cfg *config.Config) {
	edition := "普通版"
	if cfg.Pro() {
		edition = "专业版"
	}
	scheme := "http"
	if cfg.TLSCert != "" {
		scheme = "https"
	}
	fmt.Printf(`
  ____       _                _        ____  ____
 |  _ \ _ __(_)_   ____ _  ___( )___   / __ \/ ___|
 | |_) | '__| \ \ / / _' |/ __|/ __| | |  | \___ \
 |  __/| |  | |\ V / (_| | (__| (__  | |__| |___) |
 |_|   |_|  |_| \_/ \__,_|\___|\___|  \____/|____/

  PrivacyOS %s (%s)   基于 Debian 13.7 Trixie
  Web 面板: %s://%s
  数据目录: %s
  配置文件: %s
  官网标识: privacyos
`, version.Version, edition, scheme, cfg.Listen, cfg.DataDir, cfg.ConfigDir)
	if cfg.Dev {
		fmt.Printf("  开发模式: 已启用（数据位于 %s）\n", cfg.DataDir)
		if cfg.AdminPassword == "privacyos" {
			fmt.Printf("  默认管理员: %s / privacyos（请在生产环境立即修改）\n", cfg.AdminUser)
		}
	}
	fmt.Println()
}
