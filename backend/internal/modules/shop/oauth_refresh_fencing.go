package shop

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
	"github.com/trademind-ai/trademind/backend/internal/pkg/security"
	platformp "github.com/trademind-ai/trademind/backend/internal/providers/platform"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type oauthExpectedShopTokenVersionKey struct{}

type oauthRefreshAuditContext struct {
	ginContext *gin.Context
	adminID    *uuid.UUID
	action     string
}

type oauthRefreshAuditContextKey struct{}

type oauthRefreshActionKey struct{}

func withOAuthExpectedShopTokenVersion(ctx context.Context, version int64) context.Context {
	return context.WithValue(ctx, oauthExpectedShopTokenVersionKey{}, version)
}

func oauthExpectedShopTokenVersion(ctx context.Context) (int64, bool) {
	if ctx == nil {
		return 0, false
	}
	version, ok := ctx.Value(oauthExpectedShopTokenVersionKey{}).(int64)
	return version, ok && version >= 0
}

func withOAuthRefreshAuditContext(ctx context.Context, c *gin.Context, adminID *uuid.UUID, action string) context.Context {
	return context.WithValue(ctx, oauthRefreshAuditContextKey{}, oauthRefreshAuditContext{
		ginContext: c,
		adminID:    adminID,
		action:     strings.TrimSpace(action),
	})
}

func withOAuthRefreshAction(ctx context.Context, action string) context.Context {
	return context.WithValue(ctx, oauthRefreshActionKey{}, strings.TrimSpace(action))
}

func oauthRefreshAction(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	action, _ := ctx.Value(oauthRefreshActionKey{}).(string)
	return strings.TrimSpace(action)
}

func oauthRefreshAudit(ctx context.Context) (oauthRefreshAuditContext, bool) {
	if ctx == nil {
		return oauthRefreshAuditContext{}, false
	}
	audit, ok := ctx.Value(oauthRefreshAuditContextKey{}).(oauthRefreshAuditContext)
	return audit, ok && audit.ginContext != nil
}

// persistOAuthTokenRefreshVersioned fences an in-flight provider refresh with
// the token version captured before the provider request. Revoke and refresh
// lock the shop before its token row, so only one side can commit. The token
// update, authorized status, and success audit are one transaction.
func (s *Service) persistOAuthTokenRefreshVersioned(ctx context.Context, shopID uuid.UUID, access, refresh string, accessExp, refreshExp *time.Time) error {
	if s == nil || s.DB == nil {
		return fmt.Errorf("shop: no db")
	}
	if s.Encrypter == nil {
		return fmt.Errorf("shop: encryption not configured")
	}
	tenant, err := security.RequireTenantContext(ctx)
	if err != nil {
		return err
	}
	expectedVersion, ok := oauthExpectedShopTokenVersion(ctx)
	if !ok {
		expectedVersion, ok = platformp.OAuthExpectedTokenVersion(ctx)
	}
	if !ok {
		return fmt.Errorf("shop: OAuth token version binding is required")
	}
	audit, hasHTTPAudit := oauthRefreshAudit(ctx)
	action := oauthRefreshAction(ctx)
	if hasHTTPAudit && audit.action != "" {
		action = audit.action
	}
	if action == "" {
		action = "douyin.auth.refresh"
	}
	if s.OpLog == nil || s.OpLog.DB == nil {
		return fmt.Errorf("shop: OAuth refresh audit unavailable")
	}
	var actorID *uuid.UUID
	if hasHTTPAudit {
		actorID = audit.adminID
	} else if tenantContext := security.FromContext(ctx); tenantContext != nil && tenantContext.UserID != uuid.Nil {
		id := tenantContext.UserID
		actorID = &id
	}
	return s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var shopRow Shop
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND tenant_id = ?", shopID, tenant.TenantID).First(&shopRow).Error; err != nil {
			return err
		}
		var tok ShopAuthToken
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("shop_id = ?", shopID).First(&tok).Error; err != nil {
			return err
		}
		if strings.EqualFold(strings.TrimSpace(shopRow.AuthStatus), AuthUnauthorized) || tok.TokenVersion != expectedVersion {
			return fmt.Errorf("shop: OAuth token refresh is stale or revoked (expectedVersion=%d actualVersion=%d authStatus=%s)", expectedVersion, tok.TokenVersion, strings.TrimSpace(shopRow.AuthStatus))
		}
		if strings.TrimSpace(access) != "" {
			ct, err := s.Encrypter.Encrypt([]byte(strings.TrimSpace(access)))
			if err != nil {
				return err
			}
			tok.AccessTokenEnc = ct
		}
		if strings.TrimSpace(refresh) != "" {
			ct, err := s.Encrypter.Encrypt([]byte(strings.TrimSpace(refresh)))
			if err != nil {
				return err
			}
			tok.RefreshTokenEnc = ct
		}
		tok.ExpiresAt = accessExp
		tok.RefreshExpiresAt = refreshExp
		tok.TokenVersion++
		if err := tx.Save(&tok).Error; err != nil {
			return err
		}
		statusResult := tx.Model(&Shop{}).Where("id = ? AND tenant_id = ?", shopID, tenant.TenantID).Update("auth_status", AuthAuthorized)
		if statusResult.Error != nil {
			return statusResult.Error
		}
		if statusResult.RowsAffected != 1 {
			return gorm.ErrRecordNotFound
		}
		if hasHTTPAudit {
			if _, exists := audit.ginContext.Get(ctxkey.TenantID); !exists {
				audit.ginContext.Set(ctxkey.TenantID, tenant.TenantID)
			}
			return s.writeShopAuditTx(tx, audit.ginContext, audit.adminID, action, shopID.String(), "OAuth token refreshed")
		}
		return s.OpLog.WriteTxContext(ctx, tx, operationlog.WriteOpts{
			TenantID: tenant.TenantID, AdminUserID: actorID, Action: action, Resource: "shop",
			ResourceID: shopID.String(), ShopID: &shopID, Platform: shopRow.Platform, Status: "success",
			Message: "OAuth token refreshed",
		})
	})
}

// persistOAuthTokenRefreshForPlatform applies the same fence to provider
// refreshes that run from workers or non-HTTP platform operations.
func (s *Service) persistOAuthTokenRefreshForPlatform(ctx context.Context, shopID uuid.UUID, access, refresh string, accessExp, refreshExp *time.Time, action string) error {
	return s.persistOAuthTokenRefreshVersioned(withOAuthRefreshAction(ctx, action), shopID, access, refresh, accessExp, refreshExp)
}

// setAuthStatusVersionedForPlatform fences provider status changes and keeps
// their evidence in the same transaction as the status update.
func (s *Service) setAuthStatusVersionedForPlatform(ctx context.Context, shopID uuid.UUID, status, action string) error {
	if s == nil || s.DB == nil {
		return fmt.Errorf("shop: no db")
	}
	tenant, err := security.RequireTenantContext(ctx)
	if err != nil {
		return err
	}
	expectedVersion, ok := platformp.OAuthExpectedTokenVersion(ctx)
	if !ok {
		return fmt.Errorf("shop: OAuth token version binding is required")
	}
	if s.OpLog == nil || s.OpLog.DB == nil {
		return fmt.Errorf("shop: OAuth status audit unavailable")
	}
	var actorID *uuid.UUID
	if tenant.UserID != uuid.Nil {
		id := tenant.UserID
		actorID = &id
	}
	return s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var shopRow Shop
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND tenant_id = ?", shopID, tenant.TenantID).First(&shopRow).Error; err != nil {
			return err
		}
		var tok ShopAuthToken
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("shop_id = ?", shopID).First(&tok).Error; err != nil {
			return err
		}
		if tok.TokenVersion != expectedVersion || (strings.EqualFold(strings.TrimSpace(status), AuthAuthorized) && strings.EqualFold(strings.TrimSpace(shopRow.AuthStatus), AuthUnauthorized)) {
			return fmt.Errorf("shop: OAuth auth status update is stale or revoked")
		}
		result := tx.Model(&Shop{}).Where("id = ? AND tenant_id = ?", shopID, tenant.TenantID).Update("auth_status", strings.TrimSpace(status))
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return gorm.ErrRecordNotFound
		}
		return s.OpLog.WriteTxContext(ctx, tx, operationlog.WriteOpts{
			TenantID: tenant.TenantID, AdminUserID: actorID, Action: strings.TrimSpace(action), Resource: "shop",
			ResourceID: shopID.String(), ShopID: &shopID, Platform: shopRow.Platform, Status: "success",
			Message: fmt.Sprintf("OAuth auth status=%s", strings.TrimSpace(status)),
		})
	})
}

// setAuthStatusVersioned prevents a stale refresh callback from changing a
// revoked shop back to an authorized state after its credentials were cleared.
func (s *Service) setAuthStatusVersioned(ctx context.Context, shopID uuid.UUID, status string, expectedVersion int64) error {
	if s == nil || s.DB == nil {
		return fmt.Errorf("shop: no db")
	}
	tenant, err := security.RequireTenantContext(ctx)
	if err != nil {
		return err
	}
	return s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var shopRow Shop
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND tenant_id = ?", shopID, tenant.TenantID).First(&shopRow).Error; err != nil {
			return err
		}
		var tok ShopAuthToken
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("shop_id = ?", shopID).First(&tok).Error; err != nil {
			return err
		}
		if tok.TokenVersion != expectedVersion || (strings.EqualFold(strings.TrimSpace(status), AuthAuthorized) && strings.EqualFold(strings.TrimSpace(shopRow.AuthStatus), AuthUnauthorized)) {
			return fmt.Errorf("shop: OAuth auth status update is stale or revoked")
		}
		result := tx.Model(&Shop{}).Where("id = ? AND tenant_id = ?", shopID, tenant.TenantID).Update("auth_status", strings.TrimSpace(status))
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return gorm.ErrRecordNotFound
		}
		return nil
	})
}
