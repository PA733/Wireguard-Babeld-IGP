package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"mesh-backend/pkg/config"
	"mesh-backend/pkg/logger"
	"mesh-backend/pkg/server"
)

var (
	Version   = "dev"
	BuildTime = "unknown"
)

func main() {
	// 命令行参数
	configPath := flag.String("config", "configs/server.yaml", "配置文件路径")
	version := flag.Bool("version", false, "显示版本信息")
	flag.Parse()

	// 显示版本信息
	if *version {
		fmt.Printf("mesh-server version %s (built at %s)\n", Version, BuildTime)
		os.Exit(0)
	}

	// 获取工作区根目录
	workspaceRoot, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting current directory: %v\n", err)
		os.Exit(1)
	}

	// 加载服务端配置
	cfg, err := config.LoadServerConfig(*configPath, workspaceRoot)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	// 确保数据目录存在
	if err := os.MkdirAll(filepath.Dir(cfg.Storage.SQLite.Path), 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Error creating data directory: %v\n", err)
		os.Exit(1)
	}

	// 初始化日志
	logLevel := "debug"
	if !cfg.Log.Debug {
		logLevel = "info"
	}
	log, err := logger.NewLogger(cfg.Log.File, logLevel)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error initializing logger: %v\n", err)
		os.Exit(1)
	}

	// 创建并启动服务器
	srv, err := server.New(cfg, *log)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating server: %v\n", err)
		os.Exit(1)
	}

	// 启动服务器
	if err := srv.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "Error starting server: %v\n", err)
		os.Exit(1)
	}

	// 等待中断信号
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	// 优雅关闭服务器
	if err := srv.Stop(); err != nil {
		fmt.Fprintf(os.Stderr, "Error stopping server: %v\n", err)
		os.Exit(1)
	}
}
