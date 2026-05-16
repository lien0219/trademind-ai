package lazada

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	platformp "github.com/trademind-ai/trademind/backend/internal/providers/platform"
)

func ensureFreshAccess(ctx context.Context, shopID uuid.UUID, auth platformp.TestConnectionRequest) (string, platformp.TestConnectionRequest, error) {
	updated := auth
	access := strings.TrimSpace(auth.AccessToken)
	ref := strings.TrimSpace(auth.RefreshToken)
	now := time.Now().UTC()

	need := access == ""
	if !need && auth.AccessTokenExpiresAt != nil && auth.AccessTokenExpiresAt.Before(now.Add(5*time.Minute)) {
		need = true
	}
	if !need {
		return access, updated, nil
	}
	if ref == "" {
		return "", updated, fmt.Errorf("lazada authorization expired (missing refresh_token)")
	}

	tok, err := RefreshAccessToken(ctx, auth, ref)
	if err != nil {
		if shopID != uuid.Nil {
			_ = setAuthStatusMaybe(ctx, shopID, "expired")
		}
		return "", updated, err
	}
	newRef := ref
	if strings.TrimSpace(tok.RefreshToken) != "" {
		newRef = tok.RefreshToken
	}
	updated.AccessToken = tok.AccessToken
	updated.RefreshToken = newRef
	updated.AccessTokenExpiresAt = tok.AccessExpiresAt
	if tok.RefreshExpiresAt != nil {
		updated.RefreshTokenExpiresAt = tok.RefreshExpiresAt
	}
	if shopID != uuid.Nil && bridges != nil {
		_ = bridges.PersistOAuthTokenRefresh(ctx, shopID, updated.AccessToken, newRef, tok.AccessExpiresAt, tok.RefreshExpiresAt)
	}
	return updated.AccessToken, updated, nil
}

func setAuthStatusMaybe(ctx context.Context, shopID uuid.UUID, status string) error {
	if bridges == nil || shopID == uuid.Nil {
		return nil
	}
	return bridges.SetShopAuthStatus(ctx, shopID, status)
}
