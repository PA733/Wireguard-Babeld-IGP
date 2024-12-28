package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"mesh-backend/pkg/agent"
	"mesh-backend/pkg/agent/handlers"
	"mesh-backend/pkg/config"
	"mesh-backend/pkg/logger"
)

var (
	Version   = "dev"
	BuildTime = "unknown"
)

func main() {
	// 命令行参数
	configPath := flag.String("config", "configs/agent.yaml", "配置文件路径")
	version := flag.Bool("version", false, "显示版本信息")
	flag.Parse()

	// 显示版本信息
	if *version {
		fmt.Printf("mesh-agent version %s (built at %s)\n", Version, BuildTime)
		os.Exit(0)
	}

	// 获取工作区根目录
	workspaceRoot, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting current directory: %v\n", err)
		os.Exit(1)
	}

	// 加载配置
	cfg, err := config.LoadAgentConfig(*configPath, workspaceRoot)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	// 初始化日志
	log, err := logger.NewLogger(cfg.Runtime.LogPath, cfg.Runtime.LogLevel)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error initializing logger: %v\n", err)
		os.Exit(1)
	}

	// 创建Agent实例
	agent, err := agent.New(cfg, *log)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating agent: %v\n", err)
		os.Exit(1)
	}

	// 注册任务处理器
	// agent.RegisterHandler(handlers.NewUpdateHandler(cfg, *log))
	agent.RegisterHandler(handlers.NewStatusHandler(cfg, *log))

	// 启动Agent
	if err := agent.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "Error starting agent: %v\n", err)
		os.Exit(1)
	}

	log.Info().
		Str("version", Version).
		Str("build_time", BuildTime).
		Msg("Agent started successfully")

	// 等待信号
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	// 优雅关闭
	if err := agent.Stop(); err != nil {
		fmt.Fprintf(os.Stderr, "Error stopping agent: %v\n", err)
		os.Exit(1)
	}
}
