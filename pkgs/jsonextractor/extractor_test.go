package jsonextractor

import (
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"
)

func TestExtractStructuredJSON(t *testing.T) {
	jsonData := `{
		"name": "Alice",
		"age": 30,
		"skills": ["Go", "Python"],
		"profile": {
			"title": "Engineer",
			"department": "Development"
		}
	}`

	objectCount := 0
	arrayCount := 0

	err := ExtractStructuredJSON(jsonData,
		WithObjectCallback(func(data map[string]any) {
			objectCount++
			t.Logf("解析到对象: %+v\n", data)
		}),
		WithArrayCallback(func(data []any) {
			arrayCount++
			t.Logf("解析到数组: %+v\n", data)
		}),
	)

	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}

	if objectCount == 0 {
		t.Error("未解析到任何对象")
	}
	if arrayCount == 0 {
		t.Error("未解析到任何数组")
	}
}

func TestFieldStreamHandler(t *testing.T) {
	jsonData := `{
		"id": 123,
		"title": "Test",
		"content": "This is a long content field that should be streamed"
	}`

	contentReceived := false
	var wg sync.WaitGroup
	wg.Add(1)

	err := ExtractStructuredJSON(jsonData,
		WithRegisterFieldStreamHandler("content", func(key string, reader io.Reader, parents []string) {
			defer wg.Done()
			t.Logf("开始处理字段: %s", key)

			data, err := io.ReadAll(reader)
			if err != nil {
				t.Errorf("读取字段流失败: %v", err)
				return
			}

			content := strings.Trim(string(data), `"`)
			t.Logf("字段内容: %s", content)

			if strings.Contains(content, "long content") {
				contentReceived = true
			}
		}),
	)

	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	wg.Wait()

	if !contentReceived {
		t.Error("未接收到 content 字段的流数据")
	}
}

func TestKeyValueCallback(t *testing.T) {
	jsonData := `{"name": "Bob", "age": 25, "active": true}`

	kvCount := 0

	err := ExtractStructuredJSON(jsonData,
		WithRawKeyValueCallback(func(key, value any) {
			kvCount++
			t.Logf("字段 %v = %v\n", key, value)
		}),
	)

	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}

	if kvCount == 0 {
		t.Error("未接收到任何键值对")
	}
}

func TestFromStream(t *testing.T) {
	jsonData := `{"test": "data", "number": 42}`
	reader := strings.NewReader(jsonData)

	objectCount := 0

	err := ExtractStructuredJSONFromStream(reader,
		WithObjectCallback(func(data map[string]any) {
			objectCount++
			t.Logf("从流解析到对象: %+v\n", data)
		}),
	)

	if err != nil {
		t.Fatalf("流解析失败: %v", err)
	}

	if objectCount == 0 {
		t.Error("未从流中解析到任何对象")
	}
}

func ExampleExtractStructuredJSON() {
	jsonData := `{"name": "Alice", "age": 30}`

	_ = ExtractStructuredJSON(jsonData,
		WithObjectCallback(func(data map[string]any) {
			fmt.Printf("解析到对象: %+v\n", data)
		}),
	)
}

func ExampleWithRegisterFieldStreamHandler() {
	jsonData := `{"id": 123, "content": "large content..."}`

	_ = ExtractStructuredJSON(jsonData,
		WithRegisterFieldStreamHandler("content", func(key string, reader io.Reader, parents []string) {
			data, _ := io.ReadAll(reader)
			fmt.Printf("字段 %s: %s\n", key, string(data))
		}),
	)
}
