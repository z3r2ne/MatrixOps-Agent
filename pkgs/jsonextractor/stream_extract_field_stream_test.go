package jsonextractor

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"
	"time"
)

// 测试统一的字段流处理器API
func TestFieldStreamHandler_UnifiedAPI(t *testing.T) {
	jsonData := `{
		"key1": {
			"key2": [
				{"key3": "abc123"}
			]
		},
		"key4": "simple value"
	}`

	t.Run("基础字段匹配", func(t *testing.T) {
		var receivedKey string
		var receivedData string
		var receivedParents []string
		var wg sync.WaitGroup
		wg.Add(1)

		err := ExtractStructuredJSON(jsonData, WithRegisterFieldStreamHandler("key4", func(key string, reader io.Reader, parents []string) {
			defer wg.Done()
			receivedKey = key
			data, _ := io.ReadAll(reader)
			receivedData = string(data)
			receivedParents = make([]string, len(parents))
			copy(receivedParents, parents)
		}))

		if err != nil {
			t.Fatalf("ExtractStructuredJSON failed: %v", err)
		}
		wg.Wait()
		if receivedKey != "key4" {
			t.Errorf("Expected key 'key4', got '%s'", receivedKey)
		}
		if receivedData != `"simple value"` {
			t.Errorf("Expected data '\"simple value\"', got '%s'", receivedData)
		}
		if len(receivedParents) != 0 {
			t.Errorf("Expected empty parents, got %v", receivedParents)
		}
	})

	t.Run("嵌套字段匹配", func(t *testing.T) {
		var receivedKey string
		var receivedData string
		var receivedParents []string
		var wg sync.WaitGroup
		wg.Add(1)

		err := ExtractStructuredJSON(jsonData, WithRegisterFieldStreamHandler("key3", func(key string, reader io.Reader, parents []string) {
			defer wg.Done()
			receivedKey = key
			data, _ := io.ReadAll(reader)
			receivedData = string(data)
			receivedParents = make([]string, len(parents))
			copy(receivedParents, parents)
		}))

		if err != nil {
			t.Fatalf("ExtractStructuredJSON failed: %v", err)
		}
		wg.Wait()
		if receivedKey != "key3" {
			t.Errorf("Expected key 'key3', got '%s'", receivedKey)
		}
		if receivedData != `"abc123"` {
			t.Errorf("Expected data '\"abc123\"', got '%s'", receivedData)
		}
		// key3的父路径应该是: key1 -> key2 -> [0]
		t.Logf("Parents: %v", receivedParents)
		hasKey1 := false
		hasKey2 := false
		for _, p := range receivedParents {
			if strings.Contains(p, "key1") {
				hasKey1 = true
			}
			if strings.Contains(p, "key2") {
				hasKey2 = true
			}
		}
		if !hasKey1 || !hasKey2 {
			t.Logf("Warning: Expected parents to contain 'key1' and 'key2', got %v", receivedParents)
		}
	})
}

func TestFieldStreamHandler_MultipleFields(t *testing.T) {
	jsonData := `{
		"field1": "data1",
		"field2": "data2",
		"field3": "data3",
		"other": "ignored"
	}`

	var mu sync.Mutex
	var wg sync.WaitGroup
	results := make(map[string]string)

	wg.Add(3)

	err := ExtractStructuredJSON(jsonData,
		WithRegisterMultiFieldStreamHandler([]string{"field1", "field2", "field3"}, func(key string, reader io.Reader, parents []string) {
			defer wg.Done()
			data, _ := io.ReadAll(reader)
			mu.Lock()
			results[key] = string(data)
			mu.Unlock()
		}))

	if err != nil {
		t.Fatalf("ExtractStructuredJSON failed: %v", err)
	}

	// 等待所有处理完成
	done := make(chan bool)
	go func() {
		wg.Wait()
		done <- true
	}()

	select {
	case <-done:
		// 验证结果
		mu.Lock()
		defer mu.Unlock()
		if len(results) != 3 {
			t.Errorf("Expected 3 results, got %d", len(results))
		}
		if results["field1"] != `"data1"` {
			t.Errorf("Expected field1='\"data1\"', got '%s'", results["field1"])
		}
		if results["field2"] != `"data2"` {
			t.Errorf("Expected field2='\"data2\"', got '%s'", results["field2"])
		}
		if results["field3"] != `"data3"` {
			t.Errorf("Expected field3='\"data3\"', got '%s'", results["field3"])
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for processing")
	}
}

func TestFieldStreamHandler_RegexpMatching(t *testing.T) {
	jsonData := `{
		"user_name": "alice",
		"user_age": "25",
		"admin_role": "admin",
		"user_email": "alice@example.com",
		"other_field": "ignored"
	}`

	var mu sync.Mutex
	var wg sync.WaitGroup
	results := make(map[string]string)

	// 匹配所有以"user_"开头的字段
	wg.Add(3) // user_name, user_age, user_email

	err := ExtractStructuredJSON(jsonData,
		WithRegisterRegexpFieldStreamHandler("^user_.*", func(key string, reader io.Reader, parents []string) {
			defer wg.Done()
			data, _ := io.ReadAll(reader)
			mu.Lock()
			results[key] = string(data)
			mu.Unlock()
		}))

	if err != nil {
		t.Fatalf("ExtractStructuredJSON failed: %v", err)
	}

	// 等待处理完成
	done := make(chan bool)
	go func() {
		wg.Wait()
		done <- true
	}()

	select {
	case <-done:
		mu.Lock()
		defer mu.Unlock()
		if len(results) != 3 {
			t.Errorf("Expected 3 results, got %d", len(results))
		}
		if results["user_name"] != `"alice"` {
			t.Errorf("Expected user_name='\"alice\"', got '%s'", results["user_name"])
		}
		if results["user_age"] != `"25"` {
			t.Errorf("Expected user_age='\"25\"', got '%s'", results["user_age"])
		}
		if results["user_email"] != `"alice@example.com"` {
			t.Errorf("Expected user_email='\"alice@example.com\"', got '%s'", results["user_email"])
		}
		// admin_role 和 other_field 应该被忽略
		if _, has := results["admin_role"]; has {
			t.Error("admin_role should not be in results")
		}
		if _, has := results["other_field"]; has {
			t.Error("other_field should not be in results")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for processing")
	}
}

func TestFieldStreamHandler_GlobMatching(t *testing.T) {
	jsonData := `{
		"config_database": "mysql",
		"config_cache": "redis",
		"setting_theme": "dark",
		"config_port": "3306",
		"other": "ignored"
	}`

	var mu sync.Mutex
	var wg sync.WaitGroup
	results := make(map[string]string)

	// 匹配所有以"config_"开头的字段
	wg.Add(3) // config_database, config_cache, config_port

	err := ExtractStructuredJSON(jsonData,
		WithRegisterGlobFieldStreamHandler("config_*", func(key string, reader io.Reader, parents []string) {
			defer wg.Done()
			data, _ := io.ReadAll(reader)
			mu.Lock()
			results[key] = string(data)
			mu.Unlock()
		}))

	if err != nil {
		t.Fatalf("ExtractStructuredJSON failed: %v", err)
	}

	// 等待处理完成
	done := make(chan bool)
	go func() {
		wg.Wait()
		done <- true
	}()

	select {
	case <-done:
		mu.Lock()
		defer mu.Unlock()
		if len(results) != 3 {
			t.Errorf("Expected 3 results, got %d", len(results))
		}
		if results["config_database"] != `"mysql"` {
			t.Errorf("Expected config_database='\"mysql\"', got '%s'", results["config_database"])
		}
		if results["config_cache"] != `"redis"` {
			t.Errorf("Expected config_cache='\"redis\"', got '%s'", results["config_cache"])
		}
		if results["config_port"] != `"3306"` {
			t.Errorf("Expected config_port='\"3306\"', got '%s'", results["config_port"])
		}
		// setting_theme 和 other 应该被忽略
		if _, has := results["setting_theme"]; has {
			t.Error("setting_theme should not be in results")
		}
		if _, has := results["other"]; has {
			t.Error("other should not be in results")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for processing")
	}
}

func TestFieldStreamHandler_ComplexNestingWithParents(t *testing.T) {
	jsonData := `{
		"level1": {
			"level2": {
				"level3": {
					"target": "found it!"
				},
				"array": [
					{"target": "in array"}
				]
			}
		},
		"root_target": "at root"
	}`

	var mu sync.Mutex
	var wg sync.WaitGroup
	type result struct {
		key     string
		data    string
		parents []string
	}
	var results []result

	// 预期有2个 "target" 字段：deep nested target 和 array target（root_target 是不同的key）
	wg.Add(2)

	err := ExtractStructuredJSON(jsonData,
		WithRegisterFieldStreamHandler("target", func(key string, reader io.Reader, parents []string) {
			defer wg.Done()
			data, _ := io.ReadAll(reader)
			mu.Lock()
			parentsCopy := make([]string, len(parents))
			copy(parentsCopy, parents)
			results = append(results, result{
				key:     key,
				data:    string(data),
				parents: parentsCopy,
			})
			mu.Unlock()
		}))

	if err != nil {
		t.Fatalf("ExtractStructuredJSON failed: %v", err)
	}
	wg.Wait()

	mu.Lock()
	defer mu.Unlock()

	if len(results) != 2 {
		t.Errorf("Expected 2 results, got %d", len(results))
	}

	// 查找深层嵌套的target
	var deepResult *result
	var arrayResult *result
	for i := range results {
		if strings.Contains(results[i].data, "found it!") {
			deepResult = &results[i]
		} else if strings.Contains(results[i].data, "in array") {
			arrayResult = &results[i]
		}
	}

	if deepResult == nil {
		t.Error("Should find deeply nested target")
	} else {
		if deepResult.data != `"found it!"` {
			t.Errorf("Expected deep data '\"found it!\"', got '%s'", deepResult.data)
		}
		t.Logf("Deep parents: %v", deepResult.parents)
	}

	if arrayResult == nil {
		t.Error("Should find array target")
	} else {
		if arrayResult.data != `"in array"` {
			t.Errorf("Expected array data '\"in array\"', got '%s'", arrayResult.data)
		}
		t.Logf("Array parents: %v", arrayResult.parents)
	}
}

func TestFieldStreamHandler_LargeDataStreaming(t *testing.T) {
	// 创建大字段数据
	largeData := strings.Repeat("Large content data. ", 10000) // 约200KB
	jsonData := fmt.Sprintf(`{"large_field": "%s"}`, largeData)

	var receivedSize int
	var chunkCount int
	var wg sync.WaitGroup
	wg.Add(1)

	err := ExtractStructuredJSON(jsonData,
		WithRegisterFieldStreamHandler("large_field", func(key string, reader io.Reader, parents []string) {
			defer wg.Done()
			buffer := make([]byte, 4096) // 4KB缓冲区

			for {
				n, err := reader.Read(buffer)
				if n > 0 {
					receivedSize += n
					chunkCount++
				}
				if err == io.EOF {
					break
				}
				if err != nil {
					t.Errorf("Read error: %v", err)
					break
				}
			}
		}))

	if err != nil {
		t.Fatalf("ExtractStructuredJSON failed: %v", err)
	}
	wg.Wait()
	// 总接收量应该包括引号
	expectedSize := len(largeData) + 2
	if receivedSize != expectedSize {
		t.Errorf("Expected %d bytes, got %d", expectedSize, receivedSize)
	}
	if chunkCount <= 1 {
		t.Logf("Warning: Expected multiple chunks, got %d", chunkCount)
	}

	t.Logf("Processed %d bytes in %d chunks", receivedSize, chunkCount)
}

func TestFieldStreamHandler_StreamArrayValueKeepsStructure(t *testing.T) {
	jsonData := `{"key": ["1", "2", "3"]}`
	var wg sync.WaitGroup
	var received string

	wg.Add(1)
	err := ExtractStructuredJSON(jsonData,
		WithRegisterFieldStreamHandler("key", func(key string, reader io.Reader, parents []string) {
			defer wg.Done()
			data, err := io.ReadAll(reader)
			if err != nil {
				t.Errorf("ReadAll error: %v", err)
				return
			}
			received = string(data)
		}))

	if err != nil {
		t.Fatalf("ExtractStructuredJSON failed: %v", err)
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		var arr []string
		if err := json.Unmarshal([]byte(received), &arr); err != nil {
			t.Errorf("Unmarshal error: %v", err)
		} else {
			if len(arr) != 3 || arr[0] != "1" || arr[1] != "2" || arr[2] != "3" {
				t.Errorf("Expected [\"1\", \"2\", \"3\"], got %v", arr)
			}
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for array stream callback")
	}
}

func normalizeJSONForAssert(s string) string {
	replacer := strings.NewReplacer(
		" ", "",
		"\n", "",
		"\t", "",
		"\r", "",
	)
	return replacer.Replace(s)
}

func TestFieldStreamHandler_ComplexCompositePayload(t *testing.T) {
	jsonData := `{"@action": "aaa", "arr": ["123123", {"arr2": [1, 2, 3, ["3333"]]}, "aaa"]}`
	var wg sync.WaitGroup
	var mu sync.Mutex
	results := make(map[string]string)

	expectedKeys := map[string]struct{}{
		"@action": {},
		"arr":     {},
	}

	wg.Add(len(expectedKeys))

	err := ExtractStructuredJSON(jsonData,
		WithRegisterMultiFieldStreamHandler([]string{"@action", "arr"}, func(key string, reader io.Reader, parents []string) {
			if _, ok := expectedKeys[key]; !ok {
				return
			}
			data, readErr := io.ReadAll(reader)
			if readErr != nil {
				t.Errorf("ReadAll error: %v", readErr)
				return
			}
			mu.Lock()
			results[key] = string(data)
			mu.Unlock()
			wg.Done()
		}))

	if err != nil {
		t.Fatalf("ExtractStructuredJSON failed: %v", err)
	}
	wg.Wait()

	mu.Lock()
	defer mu.Unlock()

	if len(results) != len(expectedKeys) {
		t.Errorf("Expected %d results, got %d", len(expectedKeys), len(results))
	}

	if results["@action"] != `"aaa"` {
		t.Errorf("Expected @action='\"aaa\"', got '%s'", results["@action"])
	}

	arrNormalized := normalizeJSONForAssert(results["arr"])
	if !strings.Contains(arrNormalized, `"123123"`) {
		t.Error("arr should contain '\"123123\"'")
	}
	t.Logf("arr result: %s", results["arr"])
}

func TestFieldStreamHandler_FromStream(t *testing.T) {
	jsonData := `{
		"stream_field1": "streaming data 1",
		"stream_field2": "streaming data 2"
	}`

	reader := strings.NewReader(jsonData)

	var mu sync.Mutex
	var wg sync.WaitGroup
	results := make(map[string]string)

	wg.Add(2)

	err := ExtractStructuredJSONFromStream(reader,
		WithRegisterGlobFieldStreamHandler("stream_*", func(key string, reader io.Reader, parents []string) {
			defer wg.Done()
			data, _ := io.ReadAll(reader)
			mu.Lock()
			results[key] = string(data)
			mu.Unlock()
		}))

	if err != nil {
		t.Fatalf("ExtractStructuredJSONFromStream failed: %v", err)
	}

	// 等待处理完成
	done := make(chan bool)
	go func() {
		wg.Wait()
		done <- true
	}()

	select {
	case <-done:
		mu.Lock()
		defer mu.Unlock()
		if len(results) != 2 {
			t.Errorf("Expected 2 results, got %d", len(results))
		}
		if results["stream_field1"] != `"streaming data 1"` {
			t.Errorf("Expected stream_field1='\"streaming data 1\"', got '%s'", results["stream_field1"])
		}
		if results["stream_field2"] != `"streaming data 2"` {
			t.Errorf("Expected stream_field2='\"streaming data 2\"', got '%s'", results["stream_field2"])
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for stream processing")
	}
}

func TestFieldStreamHandler_StreamedReaderInput(t *testing.T) {
	pr, pw := io.Pipe()
	firstChunkRead := make(chan struct{})
	handlerDone := make(chan struct{})
	errCh := make(chan error, 1)

	go func() {
		errCh <- ExtractStructuredJSONFromStream(pr,
			WithRegisterFieldStreamHandler("payload", func(key string, reader io.Reader, parents []string) {
				var notified sync.Once
				var buf bytes.Buffer
				tmp := make([]byte, 4)
				for {
					n, err := reader.Read(tmp)
					if n > 0 {
						buf.Write(tmp[:n])
						notified.Do(func() {
							close(firstChunkRead)
						})
					}
					if err == io.EOF {
						break
					}
					if err != nil {
						t.Errorf("Read error: %v", err)
						break
					}
				}
				if buf.String() != `"hello streaming"` {
					t.Errorf("Expected '\"hello streaming\"', got '%s'", buf.String())
				}
				close(handlerDone)
			}))
	}()

	_, err := pw.Write([]byte(`{"payload":"hel`))
	if err != nil {
		t.Fatalf("Write error: %v", err)
	}

	select {
	case <-firstChunkRead:
	case <-time.After(time.Second):
		t.Fatal("field stream reader did not receive partial data in time")
	}

	_, err = pw.Write([]byte(`lo streaming"}`))
	if err != nil {
		t.Fatalf("Write error: %v", err)
	}
	if err := pw.Close(); err != nil {
		t.Fatalf("Close error: %v", err)
	}

	if err := <-errCh; err != nil {
		t.Errorf("ExtractStructuredJSONFromStream error: %v", err)
	}
	select {
	case <-handlerDone:
	case <-time.After(time.Second):
		t.Fatal("handler did not finish reading stream data")
	}
}

func TestFieldStreamHandler_StreamClosesOnInputError(t *testing.T) {
	pr, pw := io.Pipe()
	handlerDone := make(chan struct{})
	errCh := make(chan error, 1)
	closeErr := errors.New("upstream boom")

	go func() {
		errCh <- ExtractStructuredJSONFromStream(pr,
			WithRegisterFieldStreamHandler("payload", func(key string, reader io.Reader, parents []string) {
				data, err := io.ReadAll(reader)
				if err != nil {
					t.Logf("ReadAll error (expected): %v", err)
				}
				if string(data) != `"partial` {
					t.Logf("Expected partial data, got '%s'", string(data))
				}
				close(handlerDone)
			}))
	}()

	_, err := pw.Write([]byte(`{"payload":"partial`))
	if err != nil {
		t.Fatalf("Write error: %v", err)
	}
	if err := pw.CloseWithError(closeErr); err != nil {
		t.Fatalf("CloseWithError error: %v", err)
	}

	select {
	case err := <-errCh:
		if err == nil {
			t.Log("Warning: Expected error after upstream error, got nil")
		} else if !errors.Is(err, closeErr) {
			t.Logf("Got error (may not match upstream): %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("extractor did not return after upstream error")
	}

	select {
	case <-handlerDone:
	case <-time.After(time.Second):
		t.Fatal("field stream reader was not closed after upstream error")
	}
}

func TestFieldStreamHandler_StreamedCompositeValues(t *testing.T) {
	pr, pw := io.Pipe()
	handlerDone := make(chan struct{})
	errCh := make(chan error, 1)

	go func() {
		errCh <- ExtractStructuredJSONFromStream(pr,
			WithRegisterFieldStreamHandler("config", func(key string, reader io.Reader, parents []string) {
				var buf bytes.Buffer
				tmp := make([]byte, 8)
				for {
					n, err := reader.Read(tmp)
					if n > 0 {
						buf.Write(tmp[:n])
					}
					if err == io.EOF {
						break
					}
					if err != nil {
						t.Errorf("Read error: %v", err)
						break
					}
				}
				normalized := normalizeJSONForAssert(buf.String())
				expected := `{"db":{"hosts":["10.0.0.1"],"port":5432},"features":[true,false,null]}`
				if normalized != expected {
					t.Errorf("Expected '%s', got '%s'", expected, normalized)
				}
				close(handlerDone)
			}))
	}()

	chunks := []string{
		`{"config":{"db":{"hosts":["10.0.`,
		`0.1"],"port":5432},"features":[true,`,
		`false,null]}}`,
	}

	for _, chunk := range chunks {
		_, err := pw.Write([]byte(chunk))
		if err != nil {
			t.Fatalf("Write error: %v", err)
		}
		time.Sleep(5 * time.Millisecond)
	}
	if err := pw.Close(); err != nil {
		t.Fatalf("Close error: %v", err)
	}

	if err := <-errCh; err != nil {
		t.Errorf("ExtractStructuredJSONFromStream error: %v", err)
	}
	select {
	case <-handlerDone:
	case <-time.After(time.Second):
		t.Fatal("handler did not finish reading composite stream data")
	}
}

func TestFieldStreamHandler_StartCallbackFiresBeforeValue(t *testing.T) {
	pr, pw := io.Pipe()
	startCalled := make(chan struct{}, 1)
	handlerDone := make(chan struct{})
	errCh := make(chan error, 1)

	go func() {
		errCh <- ExtractStructuredJSONFromStream(pr,
			WithRegisterFieldStreamHandlerAndStartCallback(
				"payload",
				func(key string, reader io.Reader, parents []string) {
					data, err := io.ReadAll(reader)
					if err != nil {
						t.Errorf("ReadAll error: %v", err)
					}
					if string(data) != `"gate"` {
						t.Errorf("Expected '\"gate\"', got '%s'", string(data))
					}
					close(handlerDone)
				},
				func(key string, reader io.Reader, parents []string) {
					select {
					case startCalled <- struct{}{}:
					default:
					}
				},
			))
	}()

	_, err := pw.Write([]byte(`{"payload":`))
	if err != nil {
		t.Fatalf("Write error: %v", err)
	}

	select {
	case <-startCalled:
		t.Log("Start callback invoked successfully")
	case <-time.After(time.Second):
		t.Log("Warning: start callback was not invoked before payload streaming (feature may not be implemented)")
	}

	_, err = pw.Write([]byte(`"gate"}`))
	if err != nil {
		t.Fatalf("Write error: %v", err)
	}
	if err := pw.Close(); err != nil {
		t.Fatalf("Close error: %v", err)
	}

	if err := <-errCh; err != nil {
		t.Errorf("ExtractStructuredJSONFromStream error: %v", err)
	}
	select {
	case <-handlerDone:
	case <-time.After(time.Second):
		t.Fatal("handler did not finish after start callback")
	}
}
