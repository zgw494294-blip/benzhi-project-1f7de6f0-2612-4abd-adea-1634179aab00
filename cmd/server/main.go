package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"kilncurve-release/internal/application"
	"kilncurve-release/internal/store"
	"kilncurve-release/internal/web"
)

func main() {
	if err := run(); err != nil {
		log.Printf("服务退出: %v", err)
		os.Exit(1)
	}
}

func run() error {
	var addr, data string
	var selfcheck bool
	var timeout time.Duration
	flag.StringVar(&addr, "addr", "", "监听地址，必须为回环地址")
	flag.StringVar(&data, "data", ".kilncurve/state.json", "本地 JSON 快照路径")
	flag.BoolVar(&selfcheck, "selfcheck", false, "启动真实 HTTP 服务并执行完整业务冒烟")
	flag.DurationVar(&timeout, "timeout", 20*time.Second, "selfcheck 总超时")
	flag.Parse()
	resolved, err := resolveAddress(addr)
	if err != nil {
		return err
	}
	if selfcheck {
		dir, err := os.MkdirTemp("", "kilncurve-selfcheck-")
		if err != nil {
			return err
		}
		defer os.RemoveAll(dir)
		data = filepath.Join(dir, "state.json")
	}
	repo, err := store.Open(data)
	if err != nil {
		return err
	}
	service := application.NewService(repo)
	handler := web.NewHandler(service)
	listener, err := net.Listen("tcp", resolved)
	if err != nil {
		return fmt.Errorf("监听 %s: %w", resolved, err)
	}
	server := &http.Server{Handler: handler, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 20 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 1 << 20}
	serveErr := make(chan error, 1)
	go func() { serveErr <- server.Serve(listener) }()
	if selfcheck {
		return runSelfcheckServer(server, listener, serveErr, timeout)
	}
	log.Printf("窑炉曲线定版工作台已启动: http://%s", listener.Addr())
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	select {
	case <-ctx.Done():
	case err := <-serveErr:
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	return server.Shutdown(shutdownCtx)
}

func runSelfcheckServer(server *http.Server, listener net.Listener, serveErr <-chan error, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	client := &http.Client{Timeout: 5 * time.Second}
	base := "http://" + listener.Addr().String()
	err := selfcheckFlow(ctx, client, base)
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer shutdownCancel()
	shutdownErr := server.Shutdown(shutdownCtx)
	select {
	case serveResult := <-serveErr:
		if !errors.Is(serveResult, http.ErrServerClosed) && err == nil {
			err = serveResult
		}
	case <-shutdownCtx.Done():
		if err == nil {
			err = shutdownCtx.Err()
		}
	}
	if err != nil {
		return fmt.Errorf("selfcheck 失败: %w", err)
	}
	if shutdownErr != nil {
		return shutdownErr
	}
	fmt.Println("selfcheck 通过：已经由真实 HTTP 完成建题、冻结、失败试烧、整改复试、复核签发和工艺卡核验")
	return nil
}
