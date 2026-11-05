package api

import (
	"context"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/middleware"
	"github.com/trademind-ai/trademind/backend/internal/modules/admin"
	"github.com/trademind-ai/trademind/backend/internal/modules/auth"
	"github.com/trademind-ai/trademind/backend/internal/modules/inventoryread"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/modules/platformcredential"
	"github.com/trademind-ai/trademind/backend/internal/modules/productioncontrol"
	"github.com/trademind-ai/trademind/backend/internal/pkg/metrics"
	platformdouyin "github.com/trademind-ai/trademind/backend/internal/providers/platform/douyinshop"
)

// RegisterProductionRoutes mounts production capability routes on the existing API router.
func RegisterProductionRoutes(r gin.IRouter, dep *Deps) {
	if dep == nil {
		dep = &Deps{}
	}

	var metricCatalog *metrics.Catalog
	if dep.Obs != nil {
		metricCatalog = dep.Obs.Catalog
	}
	opLog := dep.OpLog
	if opLog == nil {
		opLog = &operationlog.Service{DB: dep.DB}
	}
	sessions := &auth.SessionService{
		Cfg: dep.Config, DB: dep.DB, Admins: &admin.Store{DB: dep.DB}, Metrics: metricCatalog, OpLog: opLog,
	}
	authed := r.Group("/api/v1")
	authed.Use(middleware.BearerAuthWithDB(dep.Config, dep.DB, sessions))

	controlSvc := &productioncontrol.Service{DB: dep.DB, Config: dep.Config, ProviderWriteGuard: func(ctx context.Context, shopID uuid.UUID) error {
		if guardErr := platformdouyin.GuardWorkerWithShop(ctx, shopID.String(), platformdouyin.FeatureProductDraft, true, false); guardErr != nil {
			return guardErr
		}
		return nil
	}}
	productioncontrol.Register(authed, &productioncontrol.Handler{Service: controlSvc})

	var cipher platformcredential.CredentialCipher
	if dep.Config != nil && strings.TrimSpace(dep.Config.ProductionCapabilities.LocalCredentialKey) != "" {
		if keys, err := platformcredential.NewLocalKeyProvider(dep.Config.ProductionCapabilities.LocalCredentialKeyRef, dep.Config.ProductionCapabilities.LocalCredentialKey); err == nil {
			cipher = platformcredential.AESGCMCredentialCipher{Keys: keys}
		}
	}
	credentialSvc := &platformcredential.Service{
		DB: dep.DB, Cipher: cipher, Metrics: metricCatalog, Environment: "unknown",
	}
	if dep.Config != nil {
		credentialSvc.Environment = dep.Config.AppEnv
		credentialSvc.OfflineEnabled = dep.Config.ProductionCapabilities.OfflineOAuthEnabled
		credentialSvc.RedirectAllowlist = append([]string(nil), dep.Config.ProductionCapabilities.RedirectAllowlist...)
		credentialSvc.OAuthStateTTL = dep.Config.ProductionCapabilities.OAuthStateTTL
	}
	platformcredential.Register(authed, &platformcredential.Handler{Service: credentialSvc})

	provider := &inventoryread.DouyinReadOnlyInventoryProvider{
		Config: dep.Config, Credentials: credentialSvc, Guard: controlSvc,
		Source: inventoryread.PublicationSource{DB: dep.DB},
	}
	pageSize, maxPages := 50, 100
	if dep.Config != nil {
		pageSize, maxPages = dep.Config.ProductionCapabilities.SKUPageSize, dep.Config.ProductionCapabilities.PaginationLimit
	}
	readSvc := inventoryread.NewManualReadService(dep.DB, provider, metricCatalog, pageSize, maxPages)
	if dep.Config != nil {
		readSvc.Environment = dep.Config.AppEnv
	}
	inventoryread.Register(authed, &inventoryread.Handler{Service: readSvc})
}
