# MatrixOps

一个智能的 AI 辅助开发工作流管理平台，使用 React + shadcn/ui 前端和 Golang 后端。

## 功能特性

✨ **工作区管理**
- 创建、编辑、删除工作区
- 自动验证工作区路径
- 支持多个工作区切换

🤖 **AI 任务管理**
- AI 辅助代码编写和重构
- 智能任务队列管理
- 实时任务状态追踪

📊 **看板视图**
- 任务看板管理
- AI Worker 聊天界面
- 代码变更预览

🔍 **代码审查**
- 智能代码差异对比
- AI 辅助代码审查
- 文件变更追踪

## 技术栈

### 前端
- React 19
- TypeScript
- Vite
- shadcn/ui
- Tailwind CSS
- Lucide Icons

### 后端
- Go 1.21+
- Gin Web Framework
- GORM
- SQLite

## 快速开始

### 前置要求
- Node.js 18+
- Go 1.21+
- Yarn 或 npm

### 1. 安装依赖

#### 前端
\`\`\`bash
yarn install
\`\`\`

#### 后端
\`\`\`bash
cd backend
go mod download
\`\`\`

### 2. 运行方式

#### 推荐（嵌入式）
\`\`\`bash
task start
\`\`\`

前端会构建到 `backend/web/dist`，由 Go 后端统一提供静态资源与 API。

#### 开发模式（前后端分离）

##### 启动后端（Terminal 1）
\`\`\`bash
cd backend
go run .
\`\`\`

后端服务器将在 `http://localhost:8080` 启动。

##### 启动前端（Terminal 2）
\`\`\`bash
yarn dev
\`\`\`

前端开发服务器将在 `http://localhost:3000` 启动。

### 3. 首次使用

1. 打开浏览器访问 `http://localhost:3000`
2. 首次打开会显示欢迎页面
3. 点击"创建工作区"按钮
4. 输入工作区名称
5. 选择使用默认路径或自定义路径：
   - **使用默认路径**（推荐）：自动在 `~/.matrixops/workspaces/` 下创建随机命名的工作区目录
   - **自定义路径**：手动指定已存在的目录路径
6. 选择图标和颜色
7. 点击"创建工作区"完成设置

## API 端点

### 健康检查
- `GET /health` - 服务状态检查

### 工作区管理
- `GET /api/workspaces` - 获取所有工作区
- `GET /api/workspaces/:id` - 获取单个工作区
- `POST /api/workspaces` - 创建工作区
- `PUT /api/workspaces/:id` - 更新工作区
- `DELETE /api/workspaces/:id` - 删除工作区
- `POST /api/workspaces/:id/activate` - 设置活跃工作区

## 项目结构

\`\`\`
matrixops/
├── backend/                 # Go 后端
│   ├── main.go             # 主入口
│   ├── models/             # 数据模型
│   ├── handlers/           # API 处理器
│   └── database/           # 数据库配置
├── components/             # React 组件
│   ├── ui/                 # shadcn/ui 组件
│   ├── layout/             # 全局布局与头部区
│   ├── workspaces/         # 工作区表单与对话框
│   ├── projects/           # 项目表单与对话框
│   └── workers/            # Worker 表单与对话框
├── views/                  # 视图页面
│   ├── WelcomeView.tsx    # 欢迎页面
│   ├── WorkspacesView.tsx # 工作区视图
│   ├── KanbanView.tsx     # 看板视图
│   ├── SettingsView.tsx   # 设置页面
│   ├── CodeReviewView.tsx # 代码审查
│   └── WorkbenchView.tsx  # 工作台
├── lib/                    # 工具库
│   ├── api.ts             # API 客户端
│   └── utils.ts           # 工具函数
├── App.tsx                 # 主应用
├── index.css              # 全局样式
└── types.ts               # TypeScript 类型定义
\`\`\`

## 数据存储

- **数据库文件**: `~/.matrixops/matrixops.db`
- **默认工作区**: `~/.matrixops/workspaces/{random}/`

## 开发说明

### 构建生产版本

#### 前端
\`\`\`bash
yarn build
\`\`\`

构建产物默认输出到 `backend/web/dist`，用于 Go 侧静态资源嵌入。

#### 后端
\`\`\`bash
cd backend
go build -o matrixops
\`\`\`

也可以直接使用 Taskfile：
\`\`\`bash
task build
\`\`\`

### 环境变量

创建 `.env` 文件配置 API 地址：

\`\`\`env
VITE_API_URL=http://localhost:8080/api
\`\`\`

嵌入式部署下可省略该变量，默认使用 `/api`。

## 特性说明

### 默认路径生成

- 支持自动生成工作区默认路径
- 默认路径格式: `~/.matrixops/workspaces/{随机8位字符串}`
- 自动创建工作区目录，无需手动创建
- 也支持自定义路径（需要目录已存在）

### 自动路径验证

- 在获取工作区列表时，自动检查每个工作区的路径是否存在
- 如果路径不存在，会标记错误并自动删除该工作区
- 创建工作区时会验证自定义路径是否存在

### 欢迎页面

- 首次打开应用时显示欢迎页面
- 引导用户创建第一个工作区
- 展示应用核心功能
- 提供默认路径和自定义路径两种选择

### 工作区管理

- 支持创建多个工作区
- 可自定义工作区图标和颜色
- 自动记录创建时间和更新时间
- 灵活的路径管理方式

## 贡献

欢迎提交 Issue 和 Pull Request！

## 许可证

AGPL-3.0 — 见项目根目录 [LICENSE](../LICENSE)。
