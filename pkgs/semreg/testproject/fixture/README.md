# Semreg 内置测试项目

用于 MatrixOps 测试工作区的 L1/L2 语义回归。每次跑测时会从 embed 释放到临时目录。

- `main.go` — 入口，供 explore 阅读
- `pkg/greeter/` — 简单 Go 包
- `notes.txt` — 说明文件

L2「指令遵循」用例会在本目录创建 `a.txt` 并写入 `123`。
