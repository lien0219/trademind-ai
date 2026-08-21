package database

import (
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/trademind-ai/trademind/backend/internal/modules/taskcenter"
	"gorm.io/gorm"
)

func TestMigrateTenantSecurityIndexesRebuildsTenantUniqueIndexes(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&taskcenter.TaskAlert{}, &taskcenter.TaskFailureMark{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("CREATE UNIQUE INDEX IF NOT EXISTS uq_task_alert_type_src_cat ON task_alerts (task_type, source_id, failure_category)").Error; err != nil {
		t.Fatal(err)
	}
	if err := migrateTenantSecurityIndexes(db); err != nil {
		t.Fatal(err)
	}

	var columns []struct {
		Seqno int    `gorm:"column:seqno"`
		Name  string `gorm:"column:name"`
	}
	if err := db.Raw("PRAGMA index_info(uq_task_alert_tenant_type_src_cat)").Scan(&columns).Error; err != nil {
		t.Fatal(err)
	}
	if len(columns) != 4 || columns[0].Name != "tenant_id" {
		t.Fatalf("tenant alert index columns = %#v", columns)
	}
	columns = nil
	if err := db.Raw("PRAGMA index_info(uniq_task_failure_mark_tenant)").Scan(&columns).Error; err != nil {
		t.Fatal(err)
	}
	if len(columns) != 4 || columns[0].Name != "tenant_id" {
		t.Fatalf("tenant failure mark index columns = %#v", columns)
	}
}
