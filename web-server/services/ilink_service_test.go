package services

import (
	"context"
	"testing"
	"time"

	"pkgs/db/models"
	"pkgs/testutil"
)

func TestSyncAccounts_FixesStaleOnlineStatus(t *testing.T) {
	db := testutil.OpenTaskTestDB(t)
	if err := db.AutoMigrate(&models.WechatAccount{}); err != nil {
		t.Fatalf("AutoMigrate: %v", err)
	}

	acc := models.WechatAccount{
		BotID:    "bot-1",
		BotToken: "token",
		BaseURL:  "https://example.com",
		Status:   "online",
	}
	if err := db.Create(&acc).Error; err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := db.Model(&acc).Update("enabled", false).Error; err != nil {
		t.Fatalf("Update enabled: %v", err)
	}

	s := &ILinkService{
		db:       db,
		accounts: make(map[string]*ilinkAccount),
		hub:      NewGlobalWSHub(db),
	}
	s.SyncAccounts()

	var got models.WechatAccount
	if err := db.First(&got, acc.ID).Error; err != nil {
		t.Fatalf("First: %v", err)
	}
	if got.Status != "offline" {
		t.Fatalf("status = %q, want offline", got.Status)
	}
}

func TestSyncAccounts_StartsEnabledAccountNotRunning(t *testing.T) {
	db := testutil.OpenTaskTestDB(t)
	if err := db.AutoMigrate(&models.WechatAccount{}); err != nil {
		t.Fatalf("AutoMigrate: %v", err)
	}

	acc := models.WechatAccount{
		BotID:    "bot-2",
		BotToken: "token",
		BaseURL:  "https://example.com",
		Status:   "online",
	}
	if err := db.Create(&acc).Error; err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := db.Model(&acc).Update("enabled", true).Error; err != nil {
		t.Fatalf("Update enabled: %v", err)
	}

	s := &ILinkService{
		db:       db,
		accounts: make(map[string]*ilinkAccount),
		hub:      NewGlobalWSHub(db),
	}
	s.SyncAccounts()

	deadline := time.Now().Add(2 * time.Second)
	for !s.IsAccountRunning(acc.BotID) && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	if !s.IsAccountRunning(acc.BotID) {
		t.Fatal("expected account to be marked running after sync start")
	}

	if err := db.Model(&acc).Update("enabled", false).Error; err != nil {
		t.Fatalf("disable account: %v", err)
	}
	_ = s.StopAccount(acc.BotID)
	if s.IsAccountRunning(acc.BotID) {
		t.Fatal("expected account to stop after disable")
	}
}

func TestHandleSessionExpired_DisablesAccountAndStopsRestart(t *testing.T) {
	db := testutil.OpenTaskTestDB(t)
	if err := db.AutoMigrate(&models.WechatAccount{}); err != nil {
		t.Fatalf("AutoMigrate: %v", err)
	}

	acc := models.WechatAccount{
		BotID:    "bot-expired",
		BotToken: "token",
		BaseURL:  "https://example.com",
		Status:   "online",
		Enabled:  true,
	}
	if err := db.Create(&acc).Error; err != nil {
		t.Fatalf("Create: %v", err)
	}

	_, cancel := context.WithCancel(context.Background())

	s := &ILinkService{
		db:       db,
		accounts: make(map[string]*ilinkAccount),
		hub:      NewGlobalWSHub(db),
	}
	s.accounts[acc.BotID] = &ilinkAccount{
		cancel:  cancel,
		account: &acc,
	}

	s.HandleSessionExpired(acc.BotID)

	var got models.WechatAccount
	if err := db.First(&got, acc.ID).Error; err != nil {
		t.Fatalf("First: %v", err)
	}
	if got.Enabled {
		t.Fatal("expected account to be disabled after session expiry")
	}
	if got.Status != "error" {
		t.Fatalf("status = %q, want error", got.Status)
	}
	if s.IsAccountRunning(acc.BotID) {
		t.Fatal("expected runtime to be removed after session expiry")
	}

	s.handleMonitorStopped(acc.BotID)
	if s.IsAccountRunning(acc.BotID) {
		t.Fatal("expected no auto-restart after session expiry")
	}
}

func TestUpdateAccountBinding_SyncsInMemoryBoundTaskID(t *testing.T) {
	db := testutil.OpenTaskTestDB(t)
	if err := db.AutoMigrate(&models.WechatAccount{}); err != nil {
		t.Fatalf("AutoMigrate: %v", err)
	}

	oldTaskID := uint(10)
	newTaskID := uint(20)
	acc := models.WechatAccount{
		BotID:       "bot-bind",
		BotToken:    "token",
		BaseURL:     "https://example.com",
		Status:      "online",
		Enabled:     true,
		BoundTaskID: &oldTaskID,
	}
	if err := db.Create(&acc).Error; err != nil {
		t.Fatalf("Create: %v", err)
	}

	s := &ILinkService{
		db:       db,
		accounts: make(map[string]*ilinkAccount),
		hub:      NewGlobalWSHub(db),
	}
	accountCopy := acc
	s.accounts[acc.BotID] = &ilinkAccount{account: &accountCopy}

	if err := s.UpdateAccountBinding(acc.BotID, &newTaskID); err != nil {
		t.Fatalf("UpdateAccountBinding: %v", err)
	}

	rt := s.accounts[acc.BotID]
	if rt.account.BoundTaskID == nil || *rt.account.BoundTaskID != newTaskID {
		t.Fatalf("runtime bound task = %v, want %d", rt.account.BoundTaskID, newTaskID)
	}

	var got models.WechatAccount
	if err := db.First(&got, acc.ID).Error; err != nil {
		t.Fatalf("First: %v", err)
	}
	if got.BoundTaskID == nil || *got.BoundTaskID != newTaskID {
		t.Fatalf("db bound task = %v, want %d", got.BoundTaskID, newTaskID)
	}
	if rt.listenerID == "" {
		t.Fatal("expected outbound task listener to be registered after binding change")
	}
}

func TestIsAccountRunning(t *testing.T) {
	s := &ILinkService{accounts: make(map[string]*ilinkAccount)}
	if s.IsAccountRunning("missing") {
		t.Fatal("expected missing account to be not running")
	}

	s.accounts["bot-3"] = &ilinkAccount{}
	if !s.IsAccountRunning("bot-3") {
		t.Fatal("expected account to be running")
	}
}
