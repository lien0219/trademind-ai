package shopee

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	platformp "github.com/trademind-ai/trademind/backend/internal/providers/platform"
)

func parseShopID(auth platformp.TestConnectionRequest) (int64, error) {
	s := strings.TrimSpace(auth.SellerID)
	if s == "" && auth.Extra != nil {
		s = strings.TrimSpace(auth.Extra["shop_id"])
	}
	if s == "" {
		return 0, fmt.Errorf("missing shopee shop_id (complete OAuth or set seller id)")
	}
	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil || v <= 0 {
		return 0, fmt.Errorf("invalid shopee shop_id")
	}
	return v, nil
}

func ensureFreshAccess(ctx context.Context, shopID uuid.UUID, auth platformp.TestConnectionRequest) (access string, updated platformp.TestConnectionRequest, err error) {
	updated = auth
	access = strings.TrimSpace(auth.AccessToken)
	ref := strings.TrimSpace(auth.RefreshToken)
	now := time.Now().UTC()

	sid, err := parseShopID(auth)
	if err != nil {
		return "", updated, err
	}

	need := access == ""
	if !need && auth.AccessTokenExpiresAt != nil && auth.AccessTokenExpiresAt.Before(now.Add(5*time.Minute)) {
		need = true
	}
	if !need {
		return access, updated, nil
	}
	if ref == "" {
		return "", updated, fmt.Errorf("shopee authorization expired (missing refresh_token)")
	}

	refreshCtx := ctx
	if refreshCtx == nil {
		refreshCtx = context.Background()
	}
	if shopID != uuid.Nil {
		refreshCtx = platformp.WithOAuthExpectedTokenVersion(refreshCtx, auth.TokenVersion)
	}
	tok, err := RefreshAccessToken(refreshCtx, auth, ref, sid)
	if err != nil {
		if shopID != uuid.Nil {
			if statusErr := setAuthStatusMaybe(refreshCtx, shopID, "expired"); statusErr != nil {
				return "", updated, errors.Join(err, fmt.Errorf("shopee: persist expired auth status: %w", statusErr))
			}
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

	if shopID != uuid.Nil {
		if bridges == nil {
			return "", updated, fmt.Errorf("shopee: OAuth persistence unavailable")
		}
		if err := bridges.PersistOAuthTokenRefresh(refreshCtx, shopID, updated.AccessToken, newRef, tok.AccessExpiresAt, tok.RefreshExpiresAt); err != nil {
			return "", updated, fmt.Errorf("shopee: persist refreshed token: %w", err)
		}
	}
	return updated.AccessToken, updated, nil
}

func setAuthStatusMaybe(ctx context.Context, shopID uuid.UUID, status string) error {
	if shopID == uuid.Nil {
		return nil
	}
	if bridges == nil {
		return fmt.Errorf("shopee: auth status persistence unavailable")
	}
	return bridges.SetShopAuthStatus(ctx, shopID, status)
}
