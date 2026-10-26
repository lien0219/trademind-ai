package taskcenter

import (
	"context"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/modules/settings"
	"github.com/trademind-ai/trademind/backend/internal/modules/tenantconfig"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

func TestNotifyGeneratedAlertsPropagatesSettingsReadFailure(t *testing.T) {
	service := &Service{Settings: &settings.Service{}}
	err := service.NotifyGeneratedAlerts(context.Background(), []alertNotifyCandidate{{Alert: TaskAlert{TenantID: 41, ID: uuid.New()}}}, false, nil, nil)
	require.ErrorIs(t, err, errTaskDependencyUnavailable)
}

func TestWriteNotifyOpLogPersistsTenantForBackgroundResult(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:taskcenter_notify_audit_tenant_"+uuid.NewString()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&operationlog.OperationLog{}))

	service := &Service{OpLog: &operationlog.Service{DB: db}}
	alertID := uuid.New()
	require.NoError(t, service.writeNotifyOpLog(context.Background(), nil, 41, "task_center.alert.notify.success", alertID, "webhook", "success", ""))

	var row operationlog.OperationLog
	require.NoError(t, db.Where("action = ? AND resource_id = ?", "task_center.alert.notify.success", alertID.String()).First(&row).Error)
	require.Equal(t, int64(41), row.TenantID)
}

func TestNotifyGeneratedAlertsAuditsDisabledChannelSkip(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open("file:taskcenter_notify_disabled_"+uuid.NewString()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&settings.Setting{}, &tenantconfig.Setting{}, &TaskAlertNotification{}, &operationlog.OperationLog{}))
	require.NoError(t, db.Create(&settings.Setting{TenantID: 0, GroupKey: "alert_notify", ItemKey: "enabled", ItemValue: "true"}).Error)
	require.NoError(t, db.Create(&settings.Setting{TenantID: 0, GroupKey: "alert_notify", ItemKey: "mail_enabled", ItemValue: "false"}).Error)

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("POST", "/api/v1/task-center/alerts/"+uuid.NewString()+"/notify", nil)
	c.Set(ctxkey.TenantID, int64(41))
	service := &Service{
		DB:       db,
		Settings: &settings.Service{DB: db},
		OpLog:    &operationlog.Service{DB: db},
	}
	alert := TaskAlert{ID: uuid.New(), TenantID: 41, Status: TaskAlertStatusOpen, LastSeenAt: time.Now().UTC()}

	err = service.NotifyGeneratedAlerts(context.Background(), []alertNotifyCandidate{{Alert: alert}}, true, c, []string{"mail"})
	require.NoError(t, err)

	var notification TaskAlertNotification
	require.NoError(t, db.Where("alert_id = ? AND channel = ?", alert.ID, "mail").First(&notification).Error)
	require.Equal(t, TaskAlertNotifStatusSkipped, notification.Status)
	var skipped int64
	require.NoError(t, db.Model(&operationlog.OperationLog{}).
		Where("action = ? AND status = ? AND resource_id = ?", "task_center.alert.notify.skipped", "skipped", alert.ID.String()).
		Count(&skipped).Error)
	require.Equal(t, int64(1), skipped)
}

func TestNotifyGeneratedAlertsAuditsMailValidationSkip(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open("file:taskcenter_notify_mail_validation_"+uuid.NewString()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&settings.Setting{}, &tenantconfig.Setting{}, &TaskAlertNotification{}, &operationlog.OperationLog{}))
	for _, row := range []settings.Setting{
		{TenantID: 0, GroupKey: "alert_notify", ItemKey: "enabled", ItemValue: "true"},
		{TenantID: 0, GroupKey: "alert_notify", ItemKey: "mail_enabled", ItemValue: "true"},
		{TenantID: 0, GroupKey: "alert_notify", ItemKey: "mail_to", ItemValue: "invalid recipient"},
	} {
		require.NoError(t, db.Create(&row).Error)
	}

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("POST", "/api/v1/task-center/alerts/"+uuid.NewString()+"/notify", nil)
	c.Set(ctxkey.TenantID, int64(41))
	service := &Service{
		DB:       db,
		Settings: &settings.Service{DB: db},
		OpLog:    &operationlog.Service{DB: db},
	}
	alert := TaskAlert{ID: uuid.New(), TenantID: 41, Status: TaskAlertStatusOpen, LastSeenAt: time.Now().UTC()}

	err = service.NotifyGeneratedAlerts(context.Background(), []alertNotifyCandidate{{Alert: alert}}, true, c, []string{"mail"})
	require.NoError(t, err)

	var notification TaskAlertNotification
	require.NoError(t, db.Where("alert_id = ? AND channel = ?", alert.ID, "mail").First(&notification).Error)
	require.Equal(t, TaskAlertNotifStatusSkipped, notification.Status)
	var requested, skipped int64
	require.NoError(t, db.Model(&operationlog.OperationLog{}).
		Where("action = ? AND status = ?", "task_center.alert.notify.channel.requested", "requested").
		Count(&requested).Error)
	require.NoError(t, db.Model(&operationlog.OperationLog{}).
		Where("action = ? AND status = ? AND resource_id = ?", "task_center.alert.notify.skipped", "skipped", alert.ID.String()).
		Count(&skipped).Error)
	require.Equal(t, int64(1), requested)
	require.Equal(t, int64(1), skipped)
}

func TestNotifyGeneratedAlertsWritesFailureAuditWhenNotificationPersistenceFails(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open("file:taskcenter_notify_persist_failure_"+uuid.NewString()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&settings.Setting{}, &tenantconfig.Setting{}, &TaskAlertNotification{}, &operationlog.OperationLog{}))
	for _, row := range []settings.Setting{
		{TenantID: 0, GroupKey: "alert_notify", ItemKey: "enabled", ItemValue: "true"},
		{TenantID: 0, GroupKey: "alert_notify", ItemKey: "mail_enabled", ItemValue: "true"},
		{TenantID: 0, GroupKey: "alert_notify", ItemKey: "mail_to", ItemValue: "invalid recipient"},
	} {
		require.NoError(t, db.Create(&row).Error)
	}
	require.NoError(t, db.Exec(`CREATE TRIGGER reject_task_alert_notifications BEFORE INSERT ON task_alert_notifications BEGIN SELECT RAISE(FAIL, 'notification persistence unavailable'); END`).Error)

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("POST", "/api/v1/task-center/alerts/"+uuid.NewString()+"/notify", nil)
	c.Set(ctxkey.TenantID, int64(41))
	service := &Service{
		DB:       db,
		Settings: &settings.Service{DB: db},
		OpLog:    &operationlog.Service{DB: db},
	}
	alert := TaskAlert{ID: uuid.New(), TenantID: 41, Status: TaskAlertStatusOpen, LastSeenAt: time.Now().UTC()}

	err = service.NotifyGeneratedAlerts(context.Background(), []alertNotifyCandidate{{Alert: alert}}, true, c, []string{"mail"})
	require.Error(t, err)
	require.ErrorIs(t, err, errTaskAuditOutcomeUnavailable)
	var failed int64
	require.NoError(t, db.Model(&operationlog.OperationLog{}).
		Where("action = ? AND status = ? AND resource_id = ?", "task_center.alert.notify.failed", "failed", alert.ID.String()).
		Count(&failed).Error)
	require.Equal(t, int64(1), failed)
}

func TestNotifyGeneratedAlertsAuditsDeduplicationSkip(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:taskcenter_notify_dedupe_audit_"+uuid.NewString()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&settings.Setting{}, &tenantconfig.Setting{}, &TaskAlertNotification{}, &operationlog.OperationLog{}))
	for _, row := range []settings.Setting{
		{TenantID: 0, GroupKey: "taskcenter", ItemKey: "enable_external_notifications", ItemValue: "true"},
		{TenantID: 0, GroupKey: "taskcenter", ItemKey: "notification_min_severity", ItemValue: "low"},
		{TenantID: 0, GroupKey: "taskcenter", ItemKey: "notification_channels", ItemValue: `["mail"]`},
		{TenantID: 0, GroupKey: "taskcenter", ItemKey: "notify_on_repeated_alert", ItemValue: "true"},
		{TenantID: 0, GroupKey: "alert_notify", ItemKey: "enabled", ItemValue: "true"},
		{TenantID: 0, GroupKey: "alert_notify", ItemKey: "mail_enabled", ItemValue: "true"},
	} {
		require.NoError(t, db.Create(&row).Error)
	}
	alertID := uuid.New()
	require.NoError(t, db.Create(&TaskAlertNotification{
		ID: uuid.New(), AlertID: alertID, Channel: "mail", Status: TaskAlertNotifStatusSuccess,
		RawSummary: datatypes.JSON(`{"alertCount":3}`), CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(),
	}).Error)
	service := &Service{DB: db, Settings: &settings.Service{DB: db}, OpLog: &operationlog.Service{DB: db}}

	err = service.NotifyGeneratedAlerts(context.Background(), []alertNotifyCandidate{{
		Alert: TaskAlert{ID: alertID, TenantID: 41, Status: TaskAlertStatusOpen, Severity: "high", AlertCount: 3},
	}}, false, nil, nil)
	require.NoError(t, err)

	var skipped int64
	require.NoError(t, db.Model(&operationlog.OperationLog{}).
		Where("action = ? AND status = ? AND resource_id = ?", "task_center.alert.notify.skipped", "skipped", alertID.String()).
		Count(&skipped).Error)
	require.Equal(t, int64(1), skipped)
}

func TestNotifyGeneratedAlertsRejectsManualNotificationWithoutExplicitChannels(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:taskcenter_notify_empty_channels_"+uuid.NewString()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&settings.Setting{}, &tenantconfig.Setting{}, &TaskAlertNotification{}, &operationlog.OperationLog{}))
	for _, row := range []settings.Setting{
		{TenantID: 0, GroupKey: "taskcenter", ItemKey: "notification_channels", ItemValue: `["mail"]`},
		{TenantID: 0, GroupKey: "alert_notify", ItemKey: "enabled", ItemValue: "false"},
	} {
		require.NoError(t, db.Create(&row).Error)
	}
	service := &Service{DB: db, Settings: &settings.Service{DB: db}, OpLog: &operationlog.Service{DB: db}}

	err = service.NotifyGeneratedAlerts(context.Background(), []alertNotifyCandidate{{
		Alert: TaskAlert{ID: uuid.New(), TenantID: 41},
	}}, true, nil, nil)
	require.ErrorIs(t, err, errTaskInvalidRequest)

	var notifications, audits int64
	require.NoError(t, db.Model(&TaskAlertNotification{}).Count(&notifications).Error)
	require.NoError(t, db.Model(&operationlog.OperationLog{}).Count(&audits).Error)
	require.Zero(t, notifications)
	require.Zero(t, audits)
}

func TestTenantNotificationOptOutSkipsAutomaticOutboundDelivery(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:taskcenter_tenant_notify_optout_"+uuid.NewString()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&settings.Setting{}, &tenantconfig.Setting{}, &TaskAlertNotification{}, &operationlog.OperationLog{}))
	for _, row := range []settings.Setting{
		{TenantID: 0, GroupKey: "taskcenter", ItemKey: "enable_external_notifications", ItemValue: "true"},
		{TenantID: 0, GroupKey: "taskcenter", ItemKey: "notification_min_severity", ItemValue: "low"},
		{TenantID: 0, GroupKey: "taskcenter", ItemKey: "notification_channels", ItemValue: `["mail"]`},
		{TenantID: 0, GroupKey: "alert_notify", ItemKey: "enabled", ItemValue: "true"},
		{TenantID: 0, GroupKey: "alert_notify", ItemKey: "mail_enabled", ItemValue: "true"},
	} {
		require.NoError(t, db.Create(&row).Error)
	}
	require.NoError(t, db.Create(&tenantconfig.Setting{
		TenantID: 41, Module: tenantconfig.ModuleNotifications, Version: 1,
		Values: datatypes.JSON(`{"businessAlertsEnabled":false}`), UpdatedBy: uuid.New(),
	}).Error)
	service := &Service{DB: db, Settings: &settings.Service{DB: db}, OpLog: &operationlog.Service{DB: db}}
	alert := TaskAlert{ID: uuid.New(), TenantID: 41, Status: TaskAlertStatusOpen, Severity: "high", LastSeenAt: time.Now().UTC(), AlertCount: 1}
	require.NoError(t, service.NotifyGeneratedAlerts(context.Background(), []alertNotifyCandidate{{Alert: alert, IsNew: true}}, false, nil, nil))
	var count int64
	require.NoError(t, db.Model(&TaskAlertNotification{}).Where("alert_id = ?", alert.ID).Count(&count).Error)
	require.Zero(t, count)

	enabled, version, err := tenantconfig.ReadNotifications(context.Background(), db, 42)
	require.NoError(t, err)
	require.Equal(t, int64(0), version)
	require.True(t, enabled.BusinessAlertsEnabled)
}

func TestTenantMailRecipientOverrideDoesNotUsePlatformDestination(t *testing.T) {
    db, err := gorm.Open(sqlite.Open("file:taskcenter_tenant_mail_target_"+uuid.NewString()+"?mode=memory&cache=shared"), &gorm.Config{})
    require.NoError(t, err)
    require.NoError(t, db.AutoMigrate(&settings.Setting{}, &tenantconfig.Setting{}, &TaskAlertNotification{}, &operationlog.OperationLog{}))
    for _, setting := range []settings.Setting{
        {TenantID:0,GroupKey:"taskcenter",ItemKey:"enable_external_notifications",ItemValue:"true"},
        {TenantID:0,GroupKey:"taskcenter",ItemKey:"notify_on_alert_generated",ItemValue:"true"},
        {TenantID:0,GroupKey:"taskcenter",ItemKey:"notification_min_severity",ItemValue:"low"},
        {TenantID:0,GroupKey:"taskcenter",ItemKey:"notification_channels",ItemValue:`["mail"]`},
        {TenantID:0,GroupKey:"alert_notify",ItemKey:"enabled",ItemValue:"true"},
        {TenantID:0,GroupKey:"alert_notify",ItemKey:"mail_enabled",ItemValue:"true"},
        {TenantID:0,GroupKey:"alert_notify",ItemKey:"mail_to",ItemValue:"global@example.com"},
    } {require.NoError(t,db.Create(&setting).Error)}
    require.NoError(t,db.Create(&tenantconfig.Setting{
        TenantID:51,Module:tenantconfig.ModuleNotifications,Version:1,UpdatedBy:uuid.New(),
        Values:datatypes.JSON(`{"businessAlertsEnabled":true,"mailRecipients":["enterprise@tenant.example"]}`),
    }).Error)
    service:=&Service{DB:db,Settings:&settings.Service{DB:db},OpLog:&operationlog.Service{DB:db}}
    alert:=TaskAlert{ID:uuid.New(),TenantID:51,Status:TaskAlertStatusOpen,Severity:"high",AlertCount:1,LastSeenAt:time.Now().UTC()}
    require.NoError(t,service.NotifyGeneratedAlerts(context.Background(),[]alertNotifyCandidate{{Alert:alert,IsNew:true}},false,nil,nil))
    var row TaskAlertNotification
    require.NoError(t,db.Where("alert_id=? AND channel=?",alert.ID,"mail").First(&row).Error)
    // No SMTP credentials in the fixture: delivery skips before transport, but
    // the target must be masked and tenant-specific, not the global address.
    require.Equal(t,TaskAlertNotifStatusSkipped,row.Status)
    require.Contains(t,row.Target,"tenant.example")
    require.NotContains(t,row.Target,"global@example.com")
}
