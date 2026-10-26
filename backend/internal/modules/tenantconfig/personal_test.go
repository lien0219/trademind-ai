package tenantconfig

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

func TestPersonalPreferenceIsolationAndConflict(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:personal_prefs?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&UserPreference{}); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	user := uuid.New()
	raw := json.RawMessage(`{"language":"ja-JP","theme":"dark","dateFormat":"YYYY-MM-DD"}`)
	a, err := WritePersonalPreferences(ctx, db, 41, user, raw, 0)
	if err != nil || a.Version != 1 {
		t.Fatalf("write: %#v %v", a, err)
	}
	b, err := ReadPersonalPreferences(ctx, db, 42, user)
	if err != nil || !b.Inherited {
		t.Fatalf("cross-tenant: %#v %v", b, err)
	}
	c, err := ReadPersonalPreferences(ctx, db, 41, uuid.New())
	if err != nil || !c.Inherited {
		t.Fatalf("cross-user: %#v %v", c, err)
	}
	_, err = WritePersonalPreferences(ctx, db, 41, user, raw, 0)
	if err != ErrVersionConflict {
		t.Fatalf("stale version: %v", err)
	}
	for _, language := range []string{"zh-CN", "en-US", "ja-JP", "zh-TW", "ko-KR", "th-TH"} {
		if _, err = ValidatePersonalPreferences([]byte(`{"language":"` + language + `","theme":"dark","dateFormat":"YYYY-MM-DD"}`)); err != nil {
			t.Errorf("supported language %s: %v", language, err)
		}
	}
	if _, err = ValidatePersonalPreferences([]byte(`{"language":"zh-CN","theme":"auto","dateFormat":"YYYY-MM-DD"}`)); err == nil {
		t.Fatal("invalid theme")
	}
	if _, err = ValidatePersonalPreferences([]byte(`{"language":"zh-CN","theme":"dark","dateFormat":"YYYY-MM-DD","password":"x"}`)); err == nil {
		t.Fatal("unknown key")
	}
}
