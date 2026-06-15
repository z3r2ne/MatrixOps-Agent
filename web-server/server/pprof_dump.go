package server

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	rpprof "runtime/pprof"
	"strconv"
	"strings"
	"time"

	database "pkgs/db"
)

type pprofDumpWorker struct {
	dir      string
	interval time.Duration
	stopCh   chan struct{}
	doneCh   chan struct{}
}

type memStatsSnapshot struct {
	Timestamp   string `json:"timestamp"`
	Reason      string `json:"reason"`
	Goroutines  int    `json:"goroutines"`
	Alloc       uint64 `json:"alloc"`
	TotalAlloc  uint64 `json:"totalAlloc"`
	Sys         uint64 `json:"sys"`
	HeapAlloc   uint64 `json:"heapAlloc"`
	HeapSys     uint64 `json:"heapSys"`
	HeapInuse   uint64 `json:"heapInuse"`
	HeapObjects uint64 `json:"heapObjects"`
	StackInuse  uint64 `json:"stackInuse"`
	NumGC       uint32 `json:"numGC"`
}

func startPprofDumpWorker(config ServerConfig) (*pprofDumpWorker, error) {
	if !config.EnablePprofDump {
		return nil, nil
	}

	dumpDir := strings.TrimSpace(config.PprofDumpDir)
	if dumpDir == "" {
		base, err := database.DataDir()
		if err != nil {
			return nil, fmt.Errorf("resolve pprof dump dir: %w", err)
		}
		dumpDir = filepath.Join(base, "pprof")
	}

	if err := os.MkdirAll(dumpDir, 0755); err != nil {
		return nil, fmt.Errorf("create pprof dump dir: %w", err)
	}

	interval := config.PprofDumpInterval
	if interval <= 0 {
		interval = 30 * time.Second
	}

	runDir := filepath.Join(dumpDir, time.Now().Format("20060102-150405")+"-"+strconv.Itoa(os.Getpid()))
	if err := os.MkdirAll(runDir, 0755); err != nil {
		return nil, fmt.Errorf("create pprof run dir: %w", err)
	}

	worker := &pprofDumpWorker{
		dir:      runDir,
		interval: interval,
		stopCh:   make(chan struct{}),
		doneCh:   make(chan struct{}),
	}

	if err := worker.dump("startup"); err != nil {
		log.Printf("pprof 初始 dump 失败: %v", err)
	}

	go worker.loop()
	log.Printf("📦 pprof 自动落盘已启用: %s (interval=%s)", runDir, interval)
	return worker, nil
}

func (w *pprofDumpWorker) Stop() {
	if w == nil {
		return
	}
	close(w.stopCh)
	<-w.doneCh
}

func (w *pprofDumpWorker) loop() {
	defer close(w.doneCh)

	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := w.dump("auto"); err != nil {
				log.Printf("pprof 自动 dump 失败: %v", err)
			}
		case <-w.stopCh:
			if err := w.dump("shutdown"); err != nil {
				log.Printf("pprof 关闭前 dump 失败: %v", err)
			}
			return
		}
	}
}

func (w *pprofDumpWorker) dump(reason string) error {
	stamp := time.Now().Format("20060102-150405.000")

	runtime.GC()

	profiles := []string{"heap", "allocs", "goroutine", "threadcreate"}
	for _, name := range profiles {
		if err := writeRuntimeProfile(filepath.Join(w.dir, fmt.Sprintf("%s-%s-%s.pprof", stamp, reason, name)), name); err != nil {
			return err
		}
	}

	stats := runtime.MemStats{}
	runtime.ReadMemStats(&stats)
	summary := memStatsSnapshot{
		Timestamp:   time.Now().Format(time.RFC3339Nano),
		Reason:      reason,
		Goroutines:  runtime.NumGoroutine(),
		Alloc:       stats.Alloc,
		TotalAlloc:  stats.TotalAlloc,
		Sys:         stats.Sys,
		HeapAlloc:   stats.HeapAlloc,
		HeapSys:     stats.HeapSys,
		HeapInuse:   stats.HeapInuse,
		HeapObjects: stats.HeapObjects,
		StackInuse:  stats.StackInuse,
		NumGC:       stats.NumGC,
	}

	payload, err := json.MarshalIndent(summary, "", "  ")
	if err != nil {
		return err
	}

	if err := os.WriteFile(filepath.Join(w.dir, fmt.Sprintf("%s-%s-memstats.json", stamp, reason)), payload, 0644); err != nil {
		return err
	}

	log.Printf("🧠 已写入 pprof dump: %s (%s)", w.dir, reason)
	return nil
}

func writeRuntimeProfile(path string, name string) error {
	profile := rpprof.Lookup(name)
	if profile == nil {
		return fmt.Errorf("profile not found: %s", name)
	}

	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	return profile.WriteTo(file, 0)
}
