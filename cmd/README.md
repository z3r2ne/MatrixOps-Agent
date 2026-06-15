# MatrixOps CLI

MatrixOps 的命令行工具，用于启动和管理 MatrixOps 服务器。

## 安装

```bash
# 从项目根目录编译
go build -o matrixops ./cmd/main.go

# 或使用 make（如果有 Makefile）
make build
```

## 使用方法

### 查看帮助

```bash
./matrixops --help
```

### 启动服务器

```bash
# 使用默认配置（localhost:8080）
./matrixops server

# 指定端口
./matrixops server --port 3000
./matrixops server -p 3000

# 指定主机和端口
./matrixops server --host 0.0.0.0 --port 8080
./matrixops server -H 0.0.0.0 -p 8080

# 启用详细输出
./matrixops server --verbose
./matrixops server -v

# 启用 pprof
./matrixops server --pprof

# 启用 pprof 并自动落盘
./matrixops server --pprof --pprof-dump --pprof-dump-interval 15s
```

### 环境变量

服务器启动时也支持通过环境变量配置：

```bash
# 设置端口
PORT=3000 ./matrixops server

# 设置主机
HOST=0.0.0.0 ./matrixops server

# 同时设置
HOST=0.0.0.0 PORT=8080 ./matrixops server
```

命令行参数优先级高于环境变量。

### 查看版本

```bash
./matrixops version
```

### CLI 对话

```bash
# 新建会话并获取回复
./matrixops chat "你好，帮我看看这个项目"

# 继续同一个会话
./matrixops chat --session-id session_123 "继续上一步"

# 指定 worker
./matrixops chat --worker chat "总结一下当前目录"

# 调试模式：显示 project / workdir / worker / tool 轨迹
./matrixops chat -v "看一下当前项目的相关代码"
```

## 命令说明

### `server` 命令

启动 MatrixOps Web 服务器，提供：

- REST API 接口
- WebSocket 实时通信
- Web 前端界面（如果已构建）
- 任务管理和执行

**参数：**

- `-H, --host`：服务器主机地址（默认：localhost）
- `-p, --port`：服务器端口（默认：8080）
- `--pprof`：启用 pprof 调试服务（默认监听 `localhost:6060`）
- `--pprof-dump`：启用 pprof 自动落盘
- `--pprof-dump-dir`：指定 pprof 落盘目录（默认：`~/.matrixops/pprof`）
- `--pprof-dump-interval`：指定自动落盘间隔（默认：`30s`）
- `-v, --verbose`：启用详细输出

**优雅关闭：**

服务器会监听 `SIGINT`（Ctrl+C）和 `SIGTERM` 信号，收到信号后会：

1. 停止接收新请求
2. 等待现有请求完成（最多 10 秒）
3. 清理应用程序资源
4. 关闭数据库连接
5. 退出进程

### `version` 命令

显示 MatrixOps 的版本信息。

### `chat` 命令

向 AI 发送一句话，并将回复实时打印到终端。

**参数：**

- `-s, --session-id`：已有会话 ID；为空时自动创建新会话
- `-w, --worker`：指定 worker；默认沿用会话最近一次 worker，或 `chat`
- `--project-id`：项目 ID；仅新建会话时需要，默认根据当前目录自动匹配
- `--workdir`：工作目录；默认当前目录

**行为说明：**

- 首次不传 `--session-id` 时会自动创建新会话，并输出 `session_id`
- 后续使用相同 `session_id` 即可继续对话
- 命令会一直阻塞到本轮 AI 回复结束后退出
- 默认尽量保持终端安静；加 `-v` 可看到项目、工作目录、worker 以及工具调用轨迹，便于调试

## 开发

### 项目结构

```
cmd/
├── main.go          # CLI 入口点
└── README.md        # 本文档

web-server/
├── server/          # 服务器核心逻辑
│   └── server.go
├── pkg/             # 可复用包
│   ├── app/         # 应用程序容器
│   ├── repository/  # 数据访问层
│   └── service/     # 业务逻辑层
├── handlers/        # HTTP 处理器
├── models/          # 数据模型
└── main.go         # 独立服务器入口（备用）
```

### 添加新命令

在 `cmd/main.go` 中添加新的子命令：

```go
func newYourCommand() *cobra.Command {
    return &cobra.Command{
        Use:   "your-command",
        Short: "命令简介",
        RunE: func(cmd *cobra.Command, args []string) error {
            // 命令逻辑
            return nil
        },
    }
}

// 在 main() 中注册
rootCmd.AddCommand(newYourCommand())
```

## 常见问题

### 端口已被占用

如果看到 "bind: address already in use" 错误：

```bash
# 使用其他端口
./matrixops server --port 8081

# 或查找并停止占用端口的进程
lsof -i :8080
kill -9 <PID>
```

### 数据库文件位置

数据库文件默认位置由 `web-server/database` 包决定，通常在项目根目录或 `~/.matrixops/` 下。

### 静态文件未找到

CLI 版本默认不包含静态文件。如需 Web 界面，使用 `web-server/main.go` 启动，它会嵌入构建好的前端文件。

## 贡献

欢迎提交 Issue 和 Pull Request！
