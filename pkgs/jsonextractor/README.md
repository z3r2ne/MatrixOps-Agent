# JSON Extractor

流式 JSON 解析器，从 yaklang 项目移植而来。

## 功能特性

- **流式处理**: 边读取边解析，内存高效
- **字段流处理**: 可以为特定字段注册流式处理器
- **灵活回调**: 支持对象、数组、键值对等多种回调
- **容错能力**: 能处理部分不规范的 JSON

## 使用示例

### 基本用法

```go
import "pkgs/jsonextractor"

jsonData := `{"name": "Alice", "age": 30}`

err := jsonextractor.ExtractStructuredJSON(jsonData,
    jsonextractor.WithObjectCallback(func(data map[string]any) {
        fmt.Printf("解析到对象: %+v\n", data)
    }),
)
```

### 流式处理大字段

```go
jsonData := `{"id": 123, "content": "very long content..."}`

err := jsonextractor.ExtractStructuredJSON(jsonData,
    jsonextractor.WithRegisterFieldStreamHandler("content", 
        func(key string, reader io.Reader, parents []string) {
            // 逐块读取内容，避免一次性加载到内存
            buffer := make([]byte, 1024)
            for {
                n, err := reader.Read(buffer)
                if err == io.EOF {
                    break
                }
                // 处理数据块...
                processChunk(buffer[:n])
            }
        }),
)
```

### 从流中解析

```go
file, err := os.Open("large_data.json")
if err != nil {
    return err
}
defer file.Close()

err = jsonextractor.ExtractStructuredJSONFromStream(file,
    jsonextractor.WithObjectCallback(func(data map[string]any) {
        // 处理每个对象
        processObject(data)
    }),
)
```

## API 文档

### ExtractStructuredJSON

从字符串解析 JSON。

```go
func ExtractStructuredJSON(jsonString string, options ...CallbackOption) error
```

### ExtractStructuredJSONFromStream

从数据流解析 JSON（推荐用于大文件）。

```go
func ExtractStructuredJSONFromStream(reader io.Reader, options ...CallbackOption) error
```

### 回调选项

- `WithObjectCallback` - 对象解析完成回调
- `WithArrayCallback` - 数组解析完成回调
- `WithRawKeyValueCallback` - 键值对回调
- `WithRegisterFieldStreamHandler` - 注册字段流处理器
- `WithRegisterRegexpFieldStreamHandler` - 使用正则匹配字段
- `WithRegisterGlobFieldStreamHandler` - 使用 Glob 模式匹配字段

## 与原版的区别

1. 移除了对 yaklang 特定依赖的引用
2. 使用内置的简化版 Stack 和 Pipe 实现
3. 移除了日志输出，改为静默处理
4. 保留了核心的流式解析功能

## 许可

移植自 yaklang 项目
