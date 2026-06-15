package main

import (
	"fmt"
	"os"
	"os/exec"
	"time"

	"web-server/server"

	"github.com/spf13/cobra"
)

var (
	version = "1.0.0"

	// 全局标志
	verbose bool
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "matrixops",
		Short: "MatrixOps - AI 驱动的任务管理和执行平台",
		Long: `MatrixOps 是一个强大的 AI 驱动的任务管理和执行平台。
它提供了 Web 界面来管理项目、任务和执行记录，
并支持通过 AI 代理自动化执行各种开发任务。`,
		Version:      version,
		SilenceUsage: true,
	}

	// 全局标志
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "启用详细输出")

	// 添加子命令
	rootCmd.AddCommand(newServerCommand())
	rootCmd.AddCommand(newAppCommand())
	rootCmd.AddCommand(newChatCommand())
	rootCmd.AddCommand(newVersionCommand())

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "错误: %v\n", err)
		os.Exit(1)
	}
}

// newServerCommand 创建 server 子命令
func newServerCommand() *cobra.Command {
	var (
		host                 string
		port                 string
		enablePprof          bool
		enablePprofDump      bool
		pprofDumpDir         string
		pprofDumpIntervalRaw string
	)

	cmd := &cobra.Command{
		Use:   "server",
		Short: "启动 MatrixOps Web 服务器",
		Long: `启动 MatrixOps Web 服务器。
服务器提供 REST API 和 WebSocket 连接，
用于管理工作区、项目、任务和执行记录。`,
		Example: `  # 使用默认配置启动服务器 (localhost:8080)
  matrixops server

  # 指定端口
  matrixops server --port 3000

  # 指定主机和端口
  matrixops server --host 0.0.0.0 --port 8080

  # 绑定到所有网络接口
  matrixops server -H 0.0.0.0 -p 8080

  # 启用 pprof 调试
  matrixops server --pprof

  # 启用 pprof 并自动落盘
  matrixops server --pprof --pprof-dump --pprof-dump-interval 15s`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// 从环境变量读取配置（如果命令行未指定）
			if port == "" {
				if envPort := os.Getenv("PORT"); envPort != "" {
					port = envPort
				} else {
					port = "8080"
				}
			}

			if host == "" {
				if envHost := os.Getenv("HOST"); envHost != "" {
					host = envHost
				} else {
					host = "localhost"
				}
			}

			if enablePprofDump && !enablePprof {
				enablePprof = true
			}

			pprofDumpInterval := 0 * time.Second
			if pprofDumpIntervalRaw != "" {
				parsed, err := time.ParseDuration(pprofDumpIntervalRaw)
				if err != nil {
					return fmt.Errorf("无效的 --pprof-dump-interval: %w", err)
				}
				pprofDumpInterval = parsed
			}

			if verbose {
				fmt.Printf("配置信息:\n")
				fmt.Printf("  主机: %s\n", host)
				fmt.Printf("  端口: %s\n", port)
				if enablePprof {
					fmt.Printf("  pprof: %s\n", "http://localhost:6060/debug/pprof/")
				}
				if enablePprofDump {
					fmt.Printf("  pprof dump: %s", "enabled")
					if pprofDumpDir != "" {
						fmt.Printf(" (%s)", pprofDumpDir)
					}
					if pprofDumpInterval > 0 {
						fmt.Printf(", interval=%s", pprofDumpInterval)
					}
					fmt.Println()
				}
				fmt.Println()
			}

			// 启动服务器
			config := server.ServerConfig{
				Host:              host,
				Port:              port,
				EmbeddedFiles:     nil, // CLI 不嵌入静态文件
				EnablePprof:       enablePprof,
				PprofAddr:         "localhost:6060",
				EnablePprofDump:   enablePprofDump,
				PprofDumpDir:      pprofDumpDir,
				PprofDumpInterval: pprofDumpInterval,
			}

			return server.Start(config)
		},
	}

	// 服务器标志
	cmd.Flags().StringVarP(&host, "host", "H", "", "服务器主机地址 (默认: localhost)")
	cmd.Flags().StringVarP(&port, "port", "p", "", "服务器端口 (默认: 8080)")
	cmd.Flags().BoolVar(&enablePprof, "pprof", false, "启用 pprof 调试服务 (监听 localhost:6060)")
	cmd.Flags().BoolVar(&enablePprofDump, "pprof-dump", false, "启用 pprof 自动落盘")
	cmd.Flags().StringVar(&pprofDumpDir, "pprof-dump-dir", "", "pprof 自动落盘目录 (默认: ~/.MatrixOps/pprof)")
	cmd.Flags().StringVar(&pprofDumpIntervalRaw, "pprof-dump-interval", "30s", "pprof 自动落盘间隔")

	return cmd
}

// newAppCommand 创建 app 子命令（启动 Electron 应用）
func newAppCommand() *cobra.Command {
	var (
		host string
		port string
	)

	cmd := &cobra.Command{
		Use:   "app",
		Short: "启动 MatrixOps 桌面应用（Electron）",
		Long: `启动 MatrixOps 桌面应用。
该命令会先编译前端资源（如果需要），然后启动 Electron 应用。
Electron 应用会自动启动后端服务器并在窗口中显示界面。`,
		Example: `  # 启动桌面应用（使用默认配置）
  matrixops app

  # 指定后端端口
  matrixops app --port 3000`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// 设置环境变量
			if port != "" {
				os.Setenv("PORT", port)
			}
			if host != "" {
				os.Setenv("HOST", host)
			}

			if verbose {
				fmt.Println("🚀 正在启动 MatrixOps 桌面应用...")
				if port != "" {
					fmt.Printf("  后端端口: %s\n", port)
				}
				if host != "" {
					fmt.Printf("  后端主机: %s\n", host)
				}
				fmt.Println()
			}

			// 检查 frontend 目录是否存在
			frontendDir := "frontend"
			if _, err := os.Stat(frontendDir); os.IsNotExist(err) {
				return fmt.Errorf("frontend 目录不存在: %s", frontendDir)
			}

			// 检查 node_modules 是否存在
			nodeModulesDir := frontendDir + "/node_modules"

			if _, err := os.Stat(nodeModulesDir); os.IsNotExist(err) {
				fmt.Println("⚠️  检测到依赖未安装，正在安装...")
				fmt.Println("📦 运行: npm install --legacy-peer-deps")
				fmt.Println()

				installCmd := exec.Command("npm", "install", "--legacy-peer-deps")
				installCmd.Dir = frontendDir
				installCmd.Stdout = os.Stdout
				installCmd.Stderr = os.Stderr

				if err := installCmd.Run(); err != nil {
					return fmt.Errorf("安装依赖失败: %w\n提示: 请确保已安装 Node.js 和 npm", err)
				}
				fmt.Println()
				fmt.Println("✅ 依赖安装完成")
				fmt.Println()
			}

			// 检查前端是否已构建
			distDir := "web-server/web/dist"
			if _, err := os.Stat(distDir); os.IsNotExist(err) {
				fmt.Println("⚠️  检测到前端未构建，正在构建...")
				fmt.Println("📦 运行: npm run build")
				fmt.Println()

				buildCmd := exec.Command("npm", "run", "build")
				buildCmd.Dir = frontendDir
				buildCmd.Stdout = os.Stdout
				buildCmd.Stderr = os.Stderr

				if err := buildCmd.Run(); err != nil {
					return fmt.Errorf("构建前端失败: %w", err)
				}
				fmt.Println()
				fmt.Println("✅ 前端构建完成")
				fmt.Println()
			} else if verbose {
				fmt.Println("✅ 前端已构建")
			}

			// 启动 Electron
			fmt.Println("🚀 启动 Electron 应用...")
			fmt.Println()

			electronCmd := exec.Command("npm", "run", "electron:dev")
			electronCmd.Dir = frontendDir
			electronCmd.Stdout = os.Stdout
			electronCmd.Stderr = os.Stderr
			electronCmd.Env = os.Environ()

			return electronCmd.Run()
		},
	}

	// 应用标志
	cmd.Flags().StringVarP(&host, "host", "H", "", "后端服务器主机地址 (默认: localhost)")
	cmd.Flags().StringVarP(&port, "port", "p", "", "后端服务器端口 (默认: 8080)")

	return cmd
}

// newVersionCommand 创建 version 子命令
func newVersionCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "显示版本信息",
		Long:  "显示 MatrixOps 的版本信息",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("MatrixOps v%s\n", version)
			fmt.Println("AI 驱动的任务管理和执行平台")
		},
	}
}
