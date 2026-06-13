package douyinruntime

import (
	"context"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/trademind-ai/trademind/backend/internal/modules/taskcenter"
	"gorm.io/gorm"
)

func TestUpsertDouyinAlertDedup(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&taskcenter.TaskAlert{}); err != nil {
		t.Fatal(err)
	}
	svc := &Service{DB: db}
	ctx := context.Background()
	now := time.Now().UTC()
	if err := svc.UpsertDouyinAlert(ctx, "global", AlertRateLimitSpike, "high", "title", "msg", "act", now); err != nil {
		t.Fatal(err)
	}
	if err := svc.UpsertDouyinAlert(ctx, "global", AlertRateLimitSpike, "high", "title", "msg2", "act", now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	var n int64
	_ = db.Model(&taskcenter.TaskAlert{}).Where("task_type = ? AND source_id = ? AND failure_category = ?",
		taskTypeDouyinPlatform, "global", AlertRateLimitSpike).Count(&n).Error
	if n != 1 {
		t.Fatalf("expected single deduped alert, got %d", n)
	}
	var row taskcenter.TaskAlert
	_ = db.First(&row).Error
	if row.AlertCount < 1 {
		t.Fatalf("expected alert count bump, got %d", row.AlertCount)
	}
}

func TestResolveDouyinAlert(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&taskcenter.TaskAlert{}); err != nil {
		t.Fatal(err)
	}
	svc := &Service{DB: db}
	ctx := context.Background()
	now := time.Now().UTC()
	_ = svc.UpsertDouyinAlert(ctx, "shop-1", AlertAuthExpired, "high", "t", "m", "a", now)
	if err := svc.ResolveDouyinAlert(ctx, "shop-1", AlertAuthExpired, now); err != nil {
		t.Fatal(err)
	}
	var row taskcenter.TaskAlert
	if err := db.First(&row).Error; err != nil {
		t.Fatal(err)
	}
	if row.Status != taskcenter.TaskAlertStatusResolved {
		t.Fatalf("expected resolved, got %s", row.Status)
	}
}
