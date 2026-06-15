package database

import (
	"pkgs/db/models"

	"gorm.io/gorm"
)

func GetAllMemoryLibraries(db *gorm.DB) ([]models.MemoryLibrary, error) {
	return ListMemoryLibraries(db, false, false)
}

func GetMemoryLibraryByID(db *gorm.DB, id uint) (*models.MemoryLibrary, error) {
	var library models.MemoryLibrary
	err := db.First(&library, id).Error
	return &library, err
}

func GetMemoryLibrariesByIDs(db *gorm.DB, ids []uint) ([]models.MemoryLibrary, error) {
	if len(ids) == 0 {
		return []models.MemoryLibrary{}, nil
	}
	var libraries []models.MemoryLibrary
	err := db.Where("id IN ?", ids).Find(&libraries).Error
	return libraries, err
}

func CreateMemoryLibrary(db *gorm.DB, library *models.MemoryLibrary) error {
	return db.Create(library).Error
}

func UpdateMemoryLibrary(db *gorm.DB, library *models.MemoryLibrary) error {
	return db.Save(library).Error
}

func DeleteMemoryLibrary(db *gorm.DB, id uint) error {
	return db.Delete(&models.MemoryLibrary{}, id).Error
}
