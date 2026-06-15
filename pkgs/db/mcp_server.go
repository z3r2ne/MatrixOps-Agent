package database

import (
	"strings"

	"pkgs/db/models"

	"gorm.io/gorm"
)

func ListMcpServers(db *gorm.DB) ([]models.McpServer, error) {
	var servers []models.McpServer
	err := db.Order("created_at DESC").Find(&servers).Error
	return servers, err
}

func GetMcpServerByID(db *gorm.DB, id uint) (*models.McpServer, error) {
	var server models.McpServer
	if err := db.First(&server, id).Error; err != nil {
		return nil, err
	}
	return &server, nil
}

func GetMcpServerByName(db *gorm.DB, name string) (*models.McpServer, error) {
	var server models.McpServer
	if err := db.Where("name = ?", strings.TrimSpace(name)).First(&server).Error; err != nil {
		return nil, err
	}
	return &server, nil
}

func CreateMcpServer(db *gorm.DB, server *models.McpServer) error {
	if server == nil {
		return nil
	}
	normalizeMcpServerFields(server)
	return db.Create(server).Error
}

func UpdateMcpServer(db *gorm.DB, server *models.McpServer) error {
	if server == nil {
		return nil
	}
	normalizeMcpServerFields(server)
	return db.Save(server).Error
}

func DeleteMcpServer(db *gorm.DB, id uint) error {
	return db.Delete(&models.McpServer{}, id).Error
}

func normalizeMcpServerFields(server *models.McpServer) {
	server.Name = strings.TrimSpace(server.Name)
	server.Transport = strings.TrimSpace(server.Transport)
	if server.Transport == "" {
		server.Transport = models.McpTransportStdio
	}
	server.Command = strings.TrimSpace(server.Command)
	server.URL = strings.TrimSpace(server.URL)
	if strings.TrimSpace(server.ArgsJSON) == "" {
		server.ArgsJSON = "[]"
	}
	if strings.TrimSpace(server.EnvJSON) == "" {
		server.EnvJSON = "{}"
	}
	if strings.TrimSpace(server.HeadersJSON) == "" {
		server.HeadersJSON = "{}"
	}
}
