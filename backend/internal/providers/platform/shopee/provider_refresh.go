package shopee

import (
	"context"
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

	tok, err := RefreshAccessToken(ctx, auth, ref, sid)
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
