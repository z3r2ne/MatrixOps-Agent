package models

import "time"

type SkillSource struct {
	ID            uint       `json:"id" gorm:"primaryKey"`
	Name          string     `json:"name" gorm:"not null"`
	RepoURL       string     `json:"repoUrl" gorm:"not null"`
	SkillsPath    string     `json:"skillsPath" gorm:"not null;default:'skills'"`
	Enabled       bool       `json:"enabled" gorm:"not null;default:true"`
	LocalPath     string     `json:"localPath" gorm:"type:text"`
	SkillCount    int        `json:"skillCount" gorm:"not null;default:0"`
	LastSyncAt    *time.Time `json:"lastSyncAt,omitempty"`
	LastSyncError string     `json:"lastSyncError,omitempty" gorm:"type:text"`
	CreatedAt     time.Time  `json:"createdAt"`
	UpdatedAt     time.Time  `json:"updatedAt"`
}

type SkillSourceCreate struct {
	Name       string `json:"name" binding:"required"`
	RepoURL    string `json:"repoUrl" binding:"required"`
	SkillsPath string `json:"skillsPath"`
	Enabled    *bool  `json:"enabled"`
}

type SkillSourceUpdate struct {
	Name       *string `json:"name"`
	RepoURL    *string `json:"repoUrl"`
	SkillsPath *string `json:"skillsPath"`
	Enabled    *bool   `json:"enabled"`
}

type SkillCard struct {
	ID            string `json:"id"`
	SourceID      uint   `json:"sourceId"`
	SourceName    string `json:"sourceName"`
	SourceEnabled bool   `json:"sourceEnabled"`
	Name          string `json:"name"`
	Description   string `json:"description"`
	RelativePath  string `json:"relativePath"`
	Installed     bool   `json:"installed"`
	InstalledPath string `json:"installedPath,omitempty"`
}

type SkillInstallRequest struct {
	SourceID     uint   `json:"sourceId" binding:"required"`
	RelativePath string `json:"relativePath" binding:"required"`
}
