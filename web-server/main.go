package main

import (
	"log"
	"os"

	"matrixops/server"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	host := os.Getenv("HOST")
	if host == "" {
		host = "localhost"
	}

	config := server.ServerConfig{
		Host:          host,
		Port:          port,
		EmbeddedFiles: &EmbeddedFiles,
	}

	if err := server.Start(config); err != nil {
		log.Fatalf("服务器启动失败: %v", err)
	}
}
