package taskcenter

import (
	"context"
	"errors"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/trademind-ai/trademind/backend/internal/modules/taskcenter/notify"
	"github.com/trademind-ai/trademind/backend/internal/modules/tenantconfig"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

func TestTaskAlertMailLocaleIsTenantScoped(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:task_alert_mail_locale_"+uuid.NewString()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&tenantconfig.Setting{}))
	require.NoError(t, db.Create(&tenantconfig.Setting{
		TenantID: 51, Module: tenantconfig.ModuleGeneral, Version: 1, UpdatedBy: uuid.New(),
		Values: datatypes.JSON(`{"language":"ja-JP","timezone":"Asia/Tokyo","currency":"JPY"}`),
	}).Error)
	locale, err := taskAlertMailLocale(context.Background(), db, 51)
	require.NoError(t, err)
	require.Equal(t, "ja-JP", locale)

	locale, err = taskAlertMailLocale(context.Background(), db, 52)
	require.NoError(t, err)
	require.Equal(t, "zh-CN", locale)

	locale, err = taskAlertMailLocale(context.Background(), nil, 51)
	require.Error(t, err)
	require.Empty(t, locale)
}

func TestDeliverTaskFailureTemplateUsesConfiguredLocaleAndMaskedAudit(t *testing.T) {
	var got []string
	result := deliverTaskFailureTemplate(context.Background(), "first@example.com,second@example.org", "ja-JP",
		notify.AlertNotificationPayload{
			TaskType: "inventory_sync", FailureCategory: "timeout",
			Title: "Inventory sync failed", Message: "<secret> redacted",
			DetailURL: "/tasks/123",
		},
		func(_ context.Context, event, locale, recipient string, values map[string]string) error {
			require.Equal(t, "task.failed", event)
			require.Equal(t, "ja-JP", locale)
			require.Equal(t, "inventory_sync", values["task_name"])
			require.Contains(t, values["event_name"], "/tasks/123")
			got = append(got, recipient)
			return nil
		},
	)
	require.Equal(t, TaskAlertNotifStatusSuccess, result.Status)
	require.Equal(t, "f***@example.com +1", result.Target)
	require.Equal(t, []string{"first@example.com", "second@example.org"}, got)
	require.Equal(t, "task.failed", result.RawSummary["templateEvent"])
	require.NotContains(t, result.Target, "second@example.org")
}

func TestDeliverTaskFailureTemplateRejectsInvalidAddressesAndSanitizesTransportFailures(t *testing.T) {
	called := false
	sender := func(_ context.Context, _, _, _ string, _ map[string]string) error {
		called = true
		return errors.New("sensitive SMTP transport detail")
	}
	invalid := deliverTaskFailureTemplate(context.Background(), "first@example.com,invalid", "zh-CN",
		notify.AlertNotificationPayload{}, sender)
	require.Equal(t, TaskAlertNotifStatusSkipped, invalid.Status)
	require.False(t, called)

	failed := deliverTaskFailureTemplate(context.Background(), "first@example.com", "zh-CN",
		notify.AlertNotificationPayload{TaskType: "t"}, sender)
	require.Equal(t, TaskAlertNotifStatusFailed, failed.Status)
	require.True(t, called)
	require.NotContains(t, failed.ErrorMessage, "sensitive")
	require.NotContains(t, failed.Target, "first@example.com")
}
