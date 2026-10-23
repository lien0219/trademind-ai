package auth

import (
	"bytes"
	"context"
	"fmt"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/config"
	"github.com/trademind-ai/trademind/backend/internal/modules/admin"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/pkg/authutil"
	"github.com/trademind-ai/trademind/backend/internal/pkg/model"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestRefreshTokenConcurrentRotation(t *testing.T) {
	testID := uuid.NewString()
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", testID)), &gorm.Config{})
	if err != nil {
		t.Skipf("sqlite unavailable: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	sqlDB.SetMaxOpenConns(1)
	defer sqlDB.Close()
	if err := db.AutoMigrate(&AuthSession{}, &AuthRefreshToken{}, &AuthLoginAttempt{}, &admin.AdminUser{}); err != nil {
		t.Fatal(err)
	}
	hash, _ := bcrypt.GenerateFromPassword([]byte("test-password-123"), bcrypt.DefaultCost)
	uid := uuid.New()
	email := fmt.Sprintf("test-%s@example.com", testID)
	if err := db.Create(&admin.AdminUser{
		Base:         model.Base{ID: uid},
		Username:     "testuser-" + testID,
		Email:        email,
		PasswordHash: string(hash),
		Role:         "admin",
		Status:       "active",
	}).Error; err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{
		JWTSecret: "test-jwt-secret-with-enough-length-32",
		Auth: config.AuthConfig{
			SessionMode:           config.AuthSessionModeSecure,
			AccessTokenTTLMinutes: 15,
			RefreshTokenTTLDays:   7,
		},
	}
	svc := &SessionService{Cfg: cfg, DB: db, Admins: &admin.Store{DB: db}}
	res, err := svc.CreateSession(context.Background(), email, "test-password-123", "127.0.0.1", "test")
	if err != nil {
		t.Fatal(err)
	}
	raw := res.RefreshToken
	type rotationOutcome struct {
		result *RefreshResult
		err    error
	}
	outcomes := make(chan rotationOutcome, 20)
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			result, err := svc.RotateRefresh(context.Background(), raw, "127.0.0.1", "test")
			outcomes <- rotationOutcome{result: result, err: err}
		}()
	}
	wg.Wait()
	close(outcomes)

	var successCount, reuseDenialCount, revokedDenialCount int
	var latest *RefreshResult
	for outcome := range outcomes {
		switch {
		case outcome.err == nil:
			successCount++
			latest = outcome.result
		case outcome.err.Error() == ErrRefreshTokenReused:
			reuseDenialCount++
		case outcome.err.Error() == ErrRefreshTokenRevoked:
			revokedDenialCount++
		default:
			t.Errorf("concurrent rotation returned unexpected error: %v", outcome.err)
		}
	}
	if successCount != 1 {
		t.Fatalf("expected exactly 1 successful rotation, got %d", successCount)
	}
	if reuseDenialCount != 1 {
		t.Fatalf("expected exactly 1 replay denial, got %d", reuseDenialCount)
	}
	if revokedDenialCount != 18 {
		t.Fatalf("expected 18 denials after family compromise, got %d", revokedDenialCount)
	}
	if latest == nil {
		t.Fatal("successful rotation did not return a token pair")
	}
	var active int64
	if err := db.Model(&AuthRefreshToken{}).Where("status = ?", RefreshStatusActive).Count(&active).Error; err != nil {
		t.Fatal(err)
	}
	if active != 0 {
		t.Fatalf("expected replay detection to revoke every active refresh token, got %d", active)
	}
	var session AuthSession
	if err := db.First(&session, "id = ?", res.SessionID).Error; err != nil {
		t.Fatal(err)
	}
	if session.Status != SessionStatusRevoked {
		t.Fatalf("session status = %q, want %q after refresh replay", session.Status, SessionStatusRevoked)
	}
	for name, refreshToken := range map[string]string{"original": raw, "latest": latest.RefreshToken} {
		if _, err := svc.RotateRefresh(context.Background(), refreshToken, "127.0.0.1", name); err == nil || err.Error() != ErrRefreshTokenRevoked {
			t.Fatalf("%s refresh token after replay error = %v, want %s", name, err, ErrRefreshTokenRevoked)
		}
	}
	for name, accessToken := range map[string]string{"original": res.AccessToken, "latest": latest.AccessToken} {
		claims, err := ParseAccessToken(cfg, nil, accessToken)
		if err != nil {
			t.Fatalf("parse %s access token: %v", name, err)
		}
		if err := svc.ValidateSessionAccess(context.Background(), res.SessionID, uid, claims.TokenVersion); err == nil {
			t.Fatalf("%s access token remained valid after refresh replay", name)
		}
	}
}

func TestHashTokenStable(t *testing.T) {
	a := authutil.HashToken("abc", "pepper")
	b := authutil.HashToken("abc", "pepper")
	if a != b {
		t.Fatal("hash not stable")
	}
}

func TestLoginGuardLockout(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", uuid.NewString())), &gorm.Config{})
	if err != nil {
		t.Skipf("sqlite unavailable: %v", err)
	}
	if err := db.AutoMigrate(&AuthLoginAttempt{}); err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{
		Auth: config.AuthConfig{
			LoginMaxAttempts:   3,
			LoginWindowMinutes: 15,
			AccountLockMinutes: 30,
		},
	}
	g := &LoginGuard{Cfg: cfg, DB: db}
	for i := 0; i < 3; i++ {
		_ = g.RecordFailure(context.Background(), "user@example.com", "1.2.3.4")
	}
	if err := g.CheckAllowed(context.Background(), "user@example.com", "1.2.3.4"); err == nil {
		t.Fatal("expected lockout")
	}
	_ = g.ClearFailures(context.Background(), "user@example.com", "1.2.3.4")
	if err := g.CheckAllowed(context.Background(), "user@example.com", "1.2.3.4"); err != nil {
		t.Fatal("expected unlocked after clear")
	}
}

type authQueryBudget struct {
	logger.Interface
	accountLookupCount                  int
	failedAttemptReadCount              int
	failedAttemptWriteCount             int
	operationLogPreviousHashLookupCount int
	operationLogInsertCount             int
}

func (l *authQueryBudget) LogMode(level logger.LogLevel) logger.Interface { return l }

func (l *authQueryBudget) Trace(ctx context.Context, begin time.Time, fc func() (string, int64), err error) {
	sql, _ := fc()
	s := strings.ToLower(sql)
	switch {
	case strings.Contains(s, "select") && containsTable(s, "admin_users"):
		l.accountLookupCount++
	case strings.Contains(s, "select") && containsTable(s, "auth_login_attempts"):
		l.failedAttemptReadCount++
	case strings.Contains(s, "insert into") && strings.Contains(s, "auth_login_attempts"),
		strings.Contains(s, "update") && strings.Contains(s, "auth_login_attempts"):
		l.failedAttemptWriteCount++
	case strings.Contains(s, "select") && containsTable(s, "operation_logs") && strings.Contains(s, "chain_partition"):
		l.operationLogPreviousHashLookupCount++
	case strings.Contains(s, "insert into") && strings.Contains(s, "operation_logs"):
		l.operationLogInsertCount++
	}
}

func containsTable(sql, table string) bool {
	return strings.Contains(sql, "from `"+table+"`") || strings.Contains(sql, `from "`+table+`"`) || strings.Contains(sql, "from "+table)
}

func newAuthHandlerForBudget(t testing.TB) (*Handler, *gorm.DB, *authQueryBudget) {
	t.Helper()
	budget := &authQueryBudget{Interface: logger.Default.LogMode(logger.Silent)}
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", uuid.NewString())), &gorm.Config{Logger: budget})
	if err != nil {
		t.Skipf("sqlite unavailable: %v", err)
	}
	if err := db.AutoMigrate(&AuthSession{}, &AuthRefreshToken{}, &AuthLoginAttempt{}, &admin.AdminUser{}, &operationlog.OperationLog{}); err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{
		JWTSecret: "test-jwt-secret-with-enough-length-32",
		Auth: config.AuthConfig{
			SessionMode:           config.AuthSessionModeSecure,
			AccessTokenTTLMinutes: 15,
			RefreshTokenTTLDays:   7,
			LoginMaxAttempts:      5,
			LoginWindowMinutes:    15,
			AccountLockMinutes:    30,
		},
	}
	sessions := &SessionService{Cfg: cfg, DB: db, Admins: &admin.Store{DB: db}}
	h := &Handler{
		LoginSvc: &LoginService{Cfg: cfg, Admins: &admin.Store{DB: db}, Sessions: sessions},
		Sessions: sessions,
		Admins:   &admin.Store{DB: db},
		OpLog:    &operationlog.Service{DB: db},
		DB:       db,
		Cfg:      cfg,
	}
	return h, db, budget
}

func seedAuthUser(t testing.TB, db *gorm.DB, email, password string) {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&admin.AdminUser{
		Base:         model.Base{ID: uuid.New()},
		Username:     admin.NewInternalUsername(),
		Email:        email,
		PasswordHash: string(hash),
		Role:         "admin",
		Status:       "active",
	}).Error; err != nil {
		t.Fatal(err)
	}
}

func performLogin(h *Handler, account, password string) *httptest.ResponseRecorder {
	gin.SetMode(gin.TestMode)
	body := fmt.Sprintf(`{"account":%q,"password":%q}`, account, password)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest("POST", "/api/v1/auth/login", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = "127.0.0.1:12345"
	c.Request = req
	h.Login(c)
	return w
}

func TestInvalidLoginQueryBudgetAndNoEnumeration(t *testing.T) {
	unknownH, _, unknownBudget := newAuthHandlerForBudget(t)
	unknown := performLogin(unknownH, "missing@example.com", "wrong-password")
	if unknown.Code != 401 {
		t.Fatalf("unknown account status = %d, want 401", unknown.Code)
	}
	if unknownBudget.accountLookupCount != 1 || unknownBudget.operationLogPreviousHashLookupCount != 1 || unknownBudget.operationLogInsertCount != 1 {
		t.Fatalf("unknown budget = %+v", unknownBudget)
	}

	wrongH, wrongDB, wrongBudget := newAuthHandlerForBudget(t)
	seedAuthUser(t, wrongDB, "known@example.com", "correct-password")
	wrong := performLogin(wrongH, "known@example.com", "wrong-password")
	if wrong.Code != 401 {
		t.Fatalf("wrong password status = %d, want 401", wrong.Code)
	}
	if unknown.Body.String() != wrong.Body.String() {
		t.Fatalf("unknown and wrong-password responses differ:\nunknown=%s\nwrong=%s", unknown.Body.String(), wrong.Body.String())
	}
	if wrongBudget.accountLookupCount != 1 || wrongBudget.operationLogPreviousHashLookupCount != 1 || wrongBudget.operationLogInsertCount != 1 {
		t.Fatalf("wrong-password budget = %+v", wrongBudget)
	}

	lockedH, lockedDB, lockedBudget := newAuthHandlerForBudget(t)
	lockUntil := time.Now().UTC().Add(time.Hour)
	if err := lockedDB.Create(&AuthLoginAttempt{
		AccountKey:  "locked@example.com",
		FailedCount: 5,
		LockedUntil: &lockUntil,
	}).Error; err != nil {
		t.Fatal(err)
	}
	locked := performLogin(lockedH, "locked@example.com", "any-password")
	if locked.Code != 401 {
		t.Fatalf("locked account status = %d, want existing 401 contract", locked.Code)
	}
	if lockedBudget.accountLookupCount != 0 || lockedBudget.operationLogPreviousHashLookupCount != 1 || lockedBudget.operationLogInsertCount != 1 {
		t.Fatalf("locked budget = %+v", lockedBudget)
	}
}

func BenchmarkInvalidLoginUnknownAccount(b *testing.B) {
	h, _, _ := newAuthHandlerForBudget(b)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		performLogin(h, "missing@example.com", "wrong-password")
	}
}

func BenchmarkInvalidLoginWrongPassword(b *testing.B) {
	h, db, _ := newAuthHandlerForBudget(b)
	seedAuthUser(b, db, "bench@example.com", "correct-password")
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		performLogin(h, "bench@example.com", "wrong-password")
	}
}

func BenchmarkInvalidLoginWithAudit(b *testing.B) {
	h, db, _ := newAuthHandlerForBudget(b)
	seedAuthUser(b, db, "audit@example.com", "correct-password")
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		performLogin(h, "audit@example.com", "wrong-password")
	}
}

func TestRefreshTokenDeniedAfterSecurityVersionChange(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:refresh_revoke_%s?mode=memory&cache=shared", uuid.NewString())), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&AuthSession{}, &AuthRefreshToken{}, &AuthLoginAttempt{}, &admin.AdminUser{}); err != nil {
		t.Fatal(err)
	}
	hash, err := bcrypt.GenerateFromPassword([]byte("valid-password-123"), bcrypt.DefaultCost)
	if err != nil {
		t.Fatal(err)
	}
	uid := uuid.New()
	email := "security-" + uuid.NewString() + "@example.net"
	if err := db.Create(&admin.AdminUser{
		Base:         model.Base{ID: uid},
		Username:     "security-test-" + uuid.NewString(),
		Email:        email,
		PasswordHash: string(hash),
		Role:         "operator",
		Status:       admin.StatusActive,
		TokenVersion: 1,
	}).Error; err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{
		JWTSecret: "security-test-secret-at-least-thirty-two-bytes",
		Auth: config.AuthConfig{
			SessionMode:           config.AuthSessionModeSecure,
			AccessTokenTTLMinutes: 15,
			RefreshTokenTTLDays:   7,
		},
	}
	sessions := &SessionService{Cfg: cfg, DB: db, Admins: &admin.Store{DB: db}}
	first, err := sessions.CreateSession(context.Background(), email, "valid-password-123", "127.0.0.1", "test")
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&admin.AdminUser{}).Where("id = ?", uid).
		UpdateColumn("token_version", gorm.Expr("token_version + 1")).Error; err != nil {
		t.Fatal(err)
	}
	if err := sessions.ValidateSessionAccess(context.Background(), first.SessionID, uid, 1); err == nil {
		t.Fatal("old access session accepted after revocation")
	}
	if _, err := sessions.RotateRefresh(context.Background(), first.RefreshToken, "127.0.0.1", "test"); err == nil || err.Error() != ErrSessionRevoked {
		t.Fatalf("old refresh token must not mint a new access token: %v", err)
	}
}
