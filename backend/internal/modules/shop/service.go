package shop

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/encrypt"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/modules/settings"
	platformp "github.com/trademind-ai/trademind/backend/internal/providers/platform"
	"github.com/trademind-ai/trademind/backend/internal/rdb"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// SummaryDTO is a compact shop projection for orders / conversations.
type SummaryDTO struct {
	ID         uuid.UUID `json:"id"`
	Platform   string    `json:"platform"`
	ShopName   string    `json:"shopName"`
	ShopCode   string    `json:"shopCode,omitempty"`
	Status     string    `json:"status"`
	AuthStatus string    `json:"authStatus"`
}

// Service manages shops and encrypted credentials.
type Service struct {
	DB        *gorm.DB
	Encrypter *encrypt.Service
	OpLog     *operationlog.Service
	Redis     *rdb.Client
	Settings  *settings.Service
}

// --- platform providers (public metadata) ---

// PlatformProviderDTO matches GET /platform/providers items.
type PlatformProviderDTO struct {
	Platform         string                            `json:"platform"`
	Name             string                            `json:"name"`
	Status           string                            `json:"status"`
	AuthType         string                            `json:"authType"`
	Capabilities     []string                          `json:"capabilities"`
	CapabilityStatus map[string]string                 `json:"capabilityStatus,omitempty"`
	AuthSchema       []platformp.AuthField             `json:"authSchema"`
	AuthSchemaType   string                            `json:"-"`
	AppConfigSchema  platformp.PlatformAppConfigSchema `json:"appConfigSchema"`
	SettingsGroupKey string                            `json:"settingsGroupKey"`
}

// ListPlatformProviders from registry (sorted: available first, then platform id).
func (s *Service) ListPlatformProviders() []PlatformProviderDTO {
	all := platformp.All()
	type wrap struct {
		p platformp.Provider
	}
	list := make([]wrap, 0, len(all))
	for _, p := range all {
		if p == nil {
			continue
		}
		list = append(list, wrap{p: p})
	}
	sort.Slice(list, func(i, j int) bool {
		si := list[i].p.Status()
		sj := list[j].p.Status()
		rank := func(st string) int {
			switch st {
			case platformp.StatusAvailable:
				return 0
			case platformp.StatusBeta:
				return 1
			case platformp.StatusPlanned:
				return 2
			default:
				return 9
			}
		}
		ri, rj := rank(si), rank(sj)
		if ri != rj {
			return ri < rj
		}
		return list[i].p.Platform() < list[j].p.Platform()
	})
	out := make([]PlatformProviderDTO, 0, len(list))
	for _, w := range list {
		p := w.p
		schema := p.AuthSchema()
		caps := p.Capabilities()
		cs := make([]string, len(caps))
		capSt := make(map[string]string, len(caps))
		for i := range caps {
			cs[i] = string(caps[i])
			capSt[string(caps[i])] = platformp.ImplementationStatusForCapability(p, caps[i])
		}
		fields := schema.Fields
		if fields == nil {
			fields = []platformp.AuthField{}
		}
		appSch := p.AppConfigSchema()
		out = append(out, PlatformProviderDTO{
			Platform:         p.Platform(),
			Name:             p.Name(),
			Status:           p.Status(),
			AuthType:         schema.AuthType,
			Capabilities:     cs,
			CapabilityStatus: capSt,
			AuthSchema:       fields,
			AuthSchemaType:   schema.AuthType,
			AppConfigSchema:  appSch,
			SettingsGroupKey: strings.TrimSpace(appSch.GroupKey),
		})

	}
	return out
}

// --- shops CRUD ---

type ListQuery struct {
	Page       int
	PageSize   int
	Platform   string
	Status     string
	AuthStatus string
	ShopName   string
}

type ListShopRow struct {
	ID           uuid.UUID       `json:"id"`
	Platform     string          `json:"platform"`
	ShopName     string          `json:"shopName"`
	ShopCode     string          `json:"shopCode,omitempty"`
	Status       string          `json:"status"`
	AuthStatus   string          `json:"authStatus"`
	Region       string          `json:"region,omitempty"`
	Currency     string          `json:"currency,omitempty"`
	Capabilities json.RawMessage `json:"capabilities,omitempty"`
	UpdatedAt    time.Time       `json:"updatedAt"`
}

type ListResult struct {
	Items      []ListShopRow
	Total      int64
	Page       int
	PageSize   int
	TotalPages int
}

func pagesOf(total int64, ps int) int {
	if ps < 1 {
		ps = 20
	}
	pages := int(total) / ps
	if int(total)%ps != 0 {
		pages++
	}
	if pages == 0 && total > 0 {
		pages = 1
	}
	return pages
}

func (s *Service) List(c *gin.Context, q ListQuery) (*ListResult, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("shop: no db")
	}
	page := q.Page
	if page < 1 {
		page = 1
	}
	ps := q.PageSize
	if ps < 1 {
		ps = 20
	}
	if ps > 100 {
		ps = 100
	}
	tx := s.DB.WithContext(c.Request.Context()).Model(&Shop{})
	if v := strings.TrimSpace(q.Platform); v != "" {
		tx = tx.Where("platform = ?", v)
	}
	if v := strings.TrimSpace(q.Status); v != "" {
		tx = tx.Where("status = ?", v)
	}
	if v := strings.TrimSpace(q.AuthStatus); v != "" {
		tx = tx.Where("auth_status = ?", v)
	}
	if v := strings.TrimSpace(q.ShopName); v != "" {
		tx = tx.Where("shop_name ILIKE ?", "%"+v+"%")
	}
	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, err
	}
	offset := (page - 1) * ps
	var rows []Shop
	if err := tx.Order("updated_at DESC").Offset(offset).Limit(ps).Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]ListShopRow, 0, len(rows))
	for _, r := range rows {
		var caps json.RawMessage
		if len(r.Capabilities) > 0 {
			caps = json.RawMessage(r.Capabilities)
		}
		out = append(out, ListShopRow{
			ID:           r.ID,
			Platform:     r.Platform,
			ShopName:     r.ShopName,
			ShopCode:     r.ShopCode,
			Status:       r.Status,
			AuthStatus:   r.AuthStatus,
			Region:       r.Region,
			Currency:     r.Currency,
			Capabilities: caps,
			UpdatedAt:    r.UpdatedAt,
		})
	}
	return &ListResult{Items: out, Total: total, Page: page, PageSize: ps, TotalPages: pagesOf(total, ps)}, nil
}

type CreateBody struct {
	Platform        string `json:"platform"`
	ShopName        string `json:"shopName"`
	ShopCode        string `json:"shopCode"`
	Region          string `json:"region"`
	Currency        string `json:"currency"`
	Timezone        string `json:"timezone"`
	DefaultLanguage string `json:"defaultLanguage"`
	Remark          string `json:"remark"`
}

func (s *Service) Create(c *gin.Context, body CreateBody, adminID *uuid.UUID) (*Shop, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("shop: no db")
	}
	platformID := strings.TrimSpace(body.Platform)
	if platformID == "" {
		return nil, fmt.Errorf("platform is required")
	}
	prov := platformp.Get(platformID)
	if prov == nil {
		return nil, fmt.Errorf("unknown platform %q", platformID)
	}
	name := strings.TrimSpace(body.ShopName)
	if name == "" {
		return nil, fmt.Errorf("shopName is required")
	}
	caps := prov.Capabilities()
	cs := make([]string, len(caps))
	for i := range caps {
		cs[i] = string(caps[i])
	}
	capsJSON, _ := json.Marshal(cs)

	authSt := AuthUnauthorized
	switch prov.Status() {
	case platformp.StatusAvailable:
		if platformID == "manual" {
			authSt = AuthAuthorized
		}
	default:
		// planned / beta placeholders: not yet connectable
		authSt = AuthUnauthorized
	}

	row := &Shop{
		Platform:        platformID,
		ShopName:        name,
		ShopCode:        strings.TrimSpace(body.ShopCode),
		Status:          StatusActive,
		AuthStatus:      authSt,
		Region:          strings.TrimSpace(body.Region),
		Currency:        strings.TrimSpace(strings.ToUpper(body.Currency)),
		Timezone:        strings.TrimSpace(body.Timezone),
		DefaultLanguage: strings.TrimSpace(body.DefaultLanguage),
		Capabilities:    datatypes.JSON(capsJSON),
		Remark:          strings.TrimSpace(body.Remark),
		CreatedBy:       adminID,
	}
	if err := s.DB.WithContext(c.Request.Context()).Create(row).Error; err != nil {
		return nil, err
	}
	if s.OpLog != nil {
		_ = s.OpLog.Write(c, operationlog.WriteOpts{
			AdminUserID: adminID,
			Action:      "shop.create",
			Resource:    "shop",
			ResourceID:  row.ID.String(),
			Status:      "success",
			Message:     fmt.Sprintf("shopId=%s platform=%s name=%s", row.ID.String(), row.Platform, row.ShopName),
		})
	}
	return row, nil
}

type UpdateBody struct {
	ShopName        string `json:"shopName"`
	ShopCode        string `json:"shopCode"`
	Region          string `json:"region"`
	Currency        string `json:"currency"`
	Timezone        string `json:"timezone"`
	DefaultLanguage string `json:"defaultLanguage"`
	Remark          string `json:"remark"`
	Status          string `json:"status"`
}

func (s *Service) Update(c *gin.Context, id uuid.UUID, body UpdateBody, adminID *uuid.UUID) (*Shop, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("shop: no db")
	}
	var row Shop
	if err := s.DB.WithContext(c.Request.Context()).First(&row, "id = ?", id).Error; err != nil {
		return nil, err
	}
	if v := strings.TrimSpace(body.ShopName); v != "" {
		row.ShopName = v
	}
	if v := strings.TrimSpace(body.ShopCode); v != "" {
		row.ShopCode = v
	}
	if v := strings.TrimSpace(body.Region); v != "" {
		row.Region = v
	}
	if v := strings.TrimSpace(body.Currency); v != "" {
		row.Currency = strings.ToUpper(v)
	}
	if v := strings.TrimSpace(body.Timezone); v != "" {
		row.Timezone = v
	}
	if v := strings.TrimSpace(body.DefaultLanguage); v != "" {
		row.DefaultLanguage = v
	}
	if v := strings.TrimSpace(body.Remark); v != "" {
		row.Remark = v
	}
	if st := strings.TrimSpace(body.Status); st != "" {
		if st != StatusActive && st != StatusDisabled {
			return nil, fmt.Errorf("invalid status")
		}
		row.Status = st
	}
	if err := s.DB.WithContext(c.Request.Context()).Save(&row).Error; err != nil {
		return nil, err
	}
	if s.OpLog != nil {
		_ = s.OpLog.Write(c, operationlog.WriteOpts{
			AdminUserID: adminID,
			Action:      "shop.update",
			Resource:    "shop",
			ResourceID:  row.ID.String(),
			Status:      "success",
			Message:     fmt.Sprintf("shopId=%s", row.ID.String()),
		})
	}
	return &row, nil
}

func (s *Service) Delete(c *gin.Context, id uuid.UUID, adminID *uuid.UUID) error {
	if s == nil || s.DB == nil {
		return fmt.Errorf("shop: no db")
	}
	var row Shop
	if err := s.DB.WithContext(c.Request.Context()).First(&row, "id = ?", id).Error; err != nil {
		return err
	}
	if err := s.DB.WithContext(c.Request.Context()).Delete(&row).Error; err != nil {
		return err
	}
	if s.OpLog != nil {
		_ = s.OpLog.Write(c, operationlog.WriteOpts{
			AdminUserID: adminID,
			Action:      "shop.delete",
			Resource:    "shop",
			ResourceID:  id.String(),
			Status:      "success",
			Message:     fmt.Sprintf("shopId=%s", id.String()),
		})
	}
	return nil
}

// AuthPublicDTO masks secrets for API responses.
type AuthPublicDTO struct {
	AuthType         string          `json:"authType"`
	AppKey           string          `json:"appKey,omitempty"`
	AppSecret        string          `json:"appSecret,omitempty"`
	AccessToken      string          `json:"accessToken,omitempty"`
	RefreshToken     string          `json:"refreshToken,omitempty"`
	SellerID         string          `json:"sellerId,omitempty"`
	MerchantID       string          `json:"merchantId,omitempty"`
	MarketplaceID    string          `json:"marketplaceId,omitempty"`
	ExpiresAt        *time.Time      `json:"expiresAt,omitempty"`
	RefreshExpiresAt *time.Time      `json:"refreshExpiresAt,omitempty"`
	Scopes           json.RawMessage `json:"scopes,omitempty"`
	AuthConfig       json.RawMessage `json:"authConfig,omitempty"`
}

// ShopDetailDTO is GET /shops/:id payload.
type ShopDetailDTO struct {
	Shop
	Auth *AuthPublicDTO `json:"auth,omitempty"`
}

func maskEnc(enc *encrypt.Service, stored string) string {
	if strings.TrimSpace(stored) == "" {
		return ""
	}
	if enc == nil {
		return "****"
	}
	raw, err := enc.Decrypt(stored)
	if err != nil || len(raw) == 0 {
		return "****"
	}
	return encrypt.MaskSecret(string(raw))
}

func (s *Service) buildAuthPublic(row *ShopAuthToken) *AuthPublicDTO {
	if row == nil {
		return nil
	}
	out := &AuthPublicDTO{
		AuthType:         row.AuthType,
		AppKey:           row.AppKey,
		AppSecret:        maskEnc(s.Encrypter, row.AppSecretEnc),
		AccessToken:      maskEnc(s.Encrypter, row.AccessTokenEnc),
		RefreshToken:     maskEnc(s.Encrypter, row.RefreshTokenEnc),
		SellerID:         row.SellerID,
		MerchantID:       row.MerchantID,
		MarketplaceID:    row.MarketplaceID,
		ExpiresAt:        row.ExpiresAt,
		RefreshExpiresAt: row.RefreshExpiresAt,
	}
	if len(row.Scopes) > 0 {
		out.Scopes = json.RawMessage(row.Scopes)
	}
	if len(row.AuthConfig) > 0 {
		out.AuthConfig = json.RawMessage(row.AuthConfig)
	}
	return out
}

func (s *Service) GetDetail(c *gin.Context, id uuid.UUID) (*ShopDetailDTO, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("shop: no db")
	}
	var row Shop
	if err := s.DB.WithContext(c.Request.Context()).First(&row, "id = ?", id).Error; err != nil {
		return nil, err
	}
	var tok ShopAuthToken
	err := s.DB.WithContext(c.Request.Context()).Where("shop_id = ?", id).First(&tok).Error
	var auth *AuthPublicDTO
	if err == nil {
		auth = s.buildAuthPublic(&tok)
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	return &ShopDetailDTO{Shop: row, Auth: auth}, nil
}

// GetSummary loads a shop summary or nil if missing/deleted.
func (s *Service) GetSummary(c *gin.Context, id uuid.UUID) (*SummaryDTO, error) {
	if s == nil || s.DB == nil || id == uuid.Nil {
		return nil, nil
	}
	var row Shop
	if err := s.DB.WithContext(c.Request.Context()).First(&row, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &SummaryDTO{
		ID:         row.ID,
		Platform:   row.Platform,
		ShopName:   row.ShopName,
		ShopCode:   row.ShopCode,
		Status:     row.Status,
		AuthStatus: row.AuthStatus,
	}, nil
}

// BatchSummaries returns summaries for many ids (ignores missing).
func (s *Service) BatchSummaries(c *gin.Context, ids []uuid.UUID) (map[uuid.UUID]SummaryDTO, error) {
	out := map[uuid.UUID]SummaryDTO{}
	if s == nil || s.DB == nil || len(ids) == 0 {
		return out, nil
	}
	var rows []Shop
	if err := s.DB.WithContext(c.Request.Context()).Where("id IN ?", ids).Find(&rows).Error; err != nil {
		return nil, err
	}
	for _, r := range rows {
		out[r.ID] = SummaryDTO{
			ID:         r.ID,
			Platform:   r.Platform,
			ShopName:   r.ShopName,
			ShopCode:   r.ShopCode,
			Status:     r.Status,
			AuthStatus: r.AuthStatus,
		}
	}
	return out, nil
}

// Exists returns true when shop exists and is not soft-deleted.
func (s *Service) Exists(c *gin.Context, id uuid.UUID) (bool, error) {
	if s == nil || s.DB == nil || id == uuid.Nil {
		return false, nil
	}
	var n int64
	if err := s.DB.WithContext(c.Request.Context()).Model(&Shop{}).Where("id = ?", id).Count(&n).Error; err != nil {
		return false, err
	}
	return n > 0, nil
}

type UpdateAuthBody struct {
	AuthType         string         `json:"authType"`
	AppKey           string         `json:"appKey"`
	AppSecret        string         `json:"appSecret"`
	RedirectURI      string         `json:"redirectUri"`
	AccessToken      string         `json:"accessToken"`
	RefreshToken     string         `json:"refreshToken"`
	SellerID         string         `json:"sellerId"`
	MerchantID       string         `json:"merchantId"`
	MarketplaceID    string         `json:"marketplaceId"`
	ExpiresAt        *time.Time     `json:"expiresAt"`
	RefreshExpiresAt *time.Time     `json:"refreshExpiresAt"`
	Scopes           []any          `json:"scopes"`
	AuthConfig       map[string]any `json:"authConfig"`
}

func applySecret(enc *encrypt.Service, input string, prevCipher string) (string, error) {
	if encrypt.LooksMasked(input) {
		return prevCipher, nil
	}
	input = strings.TrimSpace(input)
	if input == "" {
		return "", nil
	}
	if enc == nil {
		return "", fmt.Errorf("encryption not configured")
	}
	return enc.Encrypt([]byte(input))
}

func (s *Service) UpdateAuth(c *gin.Context, shopID uuid.UUID, body UpdateAuthBody, adminID *uuid.UUID) (*AuthPublicDTO, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("shop: no db")
	}
	var shopRow Shop
	if err := s.DB.WithContext(c.Request.Context()).First(&shopRow, "id = ?", shopID).Error; err != nil {
		return nil, err
	}
	prov := platformp.Get(shopRow.Platform)
	if prov == nil {
		return nil, fmt.Errorf("unknown platform")
	}
	if shopRow.Platform == "manual" {
		return nil, fmt.Errorf("manual platform does not require authorization storage")
	}
	schema := prov.AuthSchema()
	authTypeIn := strings.TrimSpace(body.AuthType)
	if authTypeIn == "" {
		authTypeIn = schema.AuthType
	}
	// Basic required-field validation from schema
	for _, f := range schema.Fields {
		if !f.Required {
			continue
		}
		switch f.Name {
		case "appKey":
			if strings.TrimSpace(body.AppKey) == "" {
				return nil, fmt.Errorf("field required: %s", f.Name)
			}
		case "appSecret":
			if strings.TrimSpace(body.AppSecret) == "" && !encrypt.LooksMasked(body.AppSecret) {
				return nil, fmt.Errorf("field required: %s", f.Name)
			}
		case "accessToken":
			if strings.TrimSpace(body.AccessToken) == "" && !encrypt.LooksMasked(body.AccessToken) {
				return nil, fmt.Errorf("field required: %s", f.Name)
			}
		default:
			if body.AuthConfig != nil {
				if v, ok := body.AuthConfig[f.Name]; ok {
					if fmt.Sprint(v) == "" {
						return nil, fmt.Errorf("field required: %s", f.Name)
					}
				} else {
					return nil, fmt.Errorf("field required: %s", f.Name)
				}
			} else {
				return nil, fmt.Errorf("field required: %s", f.Name)
			}
		}
	}

	var tok ShopAuthToken
	err := s.DB.WithContext(c.Request.Context()).Where("shop_id = ?", shopID).First(&tok).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		tok = ShopAuthToken{ShopID: shopID, Platform: shopRow.Platform, AuthType: authTypeIn}
		err = nil
	} else if err != nil {
		return nil, err
	}

	tok.AuthType = authTypeIn
	tok.Platform = shopRow.Platform

	if strings.EqualFold(strings.TrimSpace(shopRow.Platform), "tiktok") && strings.TrimSpace(body.RedirectURI) != "" {
		var mc map[string]any
		_ = json.Unmarshal(tok.AuthConfig, &mc)
		if mc == nil {
			mc = map[string]any{}
		}
		mc["redirectUri"] = strings.TrimSpace(body.RedirectURI)
		b, _ := json.Marshal(mc)
		tok.AuthConfig = datatypes.JSON(b)
	}

	if v := strings.TrimSpace(body.AppKey); v != "" {
		tok.AppKey = v
	}
	if ct, err := applySecret(s.Encrypter, body.AppSecret, tok.AppSecretEnc); err != nil {
		return nil, err
	} else {
		tok.AppSecretEnc = ct
	}
	if ct, err := applySecret(s.Encrypter, body.AccessToken, tok.AccessTokenEnc); err != nil {
		return nil, err
	} else {
		tok.AccessTokenEnc = ct
	}
	if ct, err := applySecret(s.Encrypter, body.RefreshToken, tok.RefreshTokenEnc); err != nil {
		return nil, err
	} else {
		tok.RefreshTokenEnc = ct
	}
	if body.SellerID != "" {
		tok.SellerID = strings.TrimSpace(body.SellerID)
	}
	if body.MerchantID != "" {
		tok.MerchantID = strings.TrimSpace(body.MerchantID)
	}
	if body.MarketplaceID != "" {
		tok.MarketplaceID = strings.TrimSpace(body.MarketplaceID)
	}
	tok.ExpiresAt = body.ExpiresAt
	tok.RefreshExpiresAt = body.RefreshExpiresAt
	if body.Scopes != nil {
		b, _ := json.Marshal(body.Scopes)
		tok.Scopes = datatypes.JSON(b)
	}
	if body.AuthConfig != nil {
		b, _ := json.Marshal(body.AuthConfig)
		tok.AuthConfig = datatypes.JSON(b)
	}
	// raw_data: only non-sensitive snapshot (no secrets)
	tok.RawData = nil

	if tok.ID == uuid.Nil {
		if err := s.DB.WithContext(c.Request.Context()).Create(&tok).Error; err != nil {
			return nil, err
		}
	} else {
		if err := s.DB.WithContext(c.Request.Context()).Save(&tok).Error; err != nil {
			return nil, err
		}
	}

	shopRow.AuthStatus = AuthAuthorized
	_ = s.DB.WithContext(c.Request.Context()).Model(&shopRow).Update("auth_status", AuthAuthorized).Error

	if s.OpLog != nil {
		_ = s.OpLog.Write(c, operationlog.WriteOpts{
			AdminUserID: adminID,
			Action:      "shop.auth.update",
			Resource:    "shop",
			ResourceID:  shopID.String(),
			Status:      "success",
			Message:     fmt.Sprintf("shopId=%s platform=%s", shopID.String(), shopRow.Platform),
		})
	}
	return s.buildAuthPublic(&tok), nil
}

// PlainAuthForProvider decrypts stored credentials for outbound Platform Provider calls (never log secrets).
func (s *Service) PlainAuthForProvider(c *gin.Context, shopID uuid.UUID) (*Shop, platformp.TestConnectionRequest, error) {
	if c == nil {
		return s.PlainAuthForProviderCtx(context.Background(), shopID)
	}
	return s.PlainAuthForProviderCtx(c.Request.Context(), shopID)
}

// PlainAuthForProviderCtx is PlainAuthForProvider for background workers (no *gin.Context).
func (s *Service) PlainAuthForProviderCtx(ctx context.Context, shopID uuid.UUID) (*Shop, platformp.TestConnectionRequest, error) {
	shopRow, _, req, err := s.decryptedAuthCtx(ctx, shopID)
	if err != nil {
		return nil, platformp.TestConnectionRequest{}, err
	}
	return shopRow, req, nil
}

// decryptedAuth builds a TestConnectionRequest from DB (for test-connection only).
func (s *Service) decryptedAuth(c *gin.Context, shopID uuid.UUID) (*Shop, *ShopAuthToken, platformp.TestConnectionRequest, error) {
	if c == nil {
		return s.decryptedAuthCtx(context.Background(), shopID)
	}
	return s.decryptedAuthCtx(c.Request.Context(), shopID)
}

func (s *Service) decryptedAuthCtx(ctx context.Context, shopID uuid.UUID) (*Shop, *ShopAuthToken, platformp.TestConnectionRequest, error) {
	var shopRow Shop
	if err := s.DB.WithContext(ctx).First(&shopRow, "id = ?", shopID).Error; err != nil {
		return nil, nil, platformp.TestConnectionRequest{}, err
	}
	var tok ShopAuthToken
	if err := s.DB.WithContext(ctx).Where("shop_id = ?", shopID).First(&tok).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return &shopRow, nil, platformp.TestConnectionRequest{}, nil
		}
		return nil, nil, platformp.TestConnectionRequest{}, err
	}
	dec := func(blob string) string {
		if s.Encrypter == nil || strings.TrimSpace(blob) == "" {
			return ""
		}
		b, err := s.Encrypter.Decrypt(blob)
		if err != nil {
			return ""
		}
		return string(b)
	}
	req := platformp.TestConnectionRequest{
		AuthType:              tok.AuthType,
		AppKey:                tok.AppKey,
		AppSecret:             dec(tok.AppSecretEnc),
		AccessToken:           dec(tok.AccessTokenEnc),
		RefreshToken:          dec(tok.RefreshTokenEnc),
		SellerID:              tok.SellerID,
		MerchantID:            tok.MerchantID,
		MarketplaceID:         tok.MarketplaceID,
		AccessTokenExpiresAt:  tok.ExpiresAt,
		RefreshTokenExpiresAt: tok.RefreshExpiresAt,
	}
	if len(tok.AuthConfig) > 0 {
		var m map[string]any
		_ = json.Unmarshal(tok.AuthConfig, &m)
		if m != nil {
			req.Extra = map[string]string{}
			for k, v := range m {
				req.Extra[k] = fmt.Sprint(v)
			}
		}
	}
	return &shopRow, &tok, req, nil
}

func (s *Service) TestConnection(c *gin.Context, shopID uuid.UUID, adminID *uuid.UUID) (*platformp.TestConnectionResult, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("shop: no db")
	}
	shopRow, _, req, err := s.decryptedAuth(c, shopID)
	if err != nil {
		return nil, err
	}
	p := platformp.Get(shopRow.Platform)
	if p == nil {
		return nil, fmt.Errorf("unknown platform")
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 15*time.Second)
	defer cancel()

	tryLog := func(res *platformp.TestConnectionResult, err error) {
		if s.OpLog == nil {
			return
		}
		action := "shop.test_connection.success"
		st := "success"
		msg := "ok"
		if err != nil {
			action = "shop.test_connection.failed"
			st = "failed"
			msg = err.Error()
		} else if res != nil && res.Message != "" {
			msg = res.Message
		}
		_ = s.OpLog.Write(c, operationlog.WriteOpts{
			AdminUserID: adminID,
			Action:      action,
			Resource:    "shop",
			ResourceID:  shopID.String(),
			Status:      st,
			Message:     fmt.Sprintf("shopId=%s platform=%s %s", shopID.String(), shopRow.Platform, msg),
		})
	}

	// Manual: no token row needed
	if shopRow.Platform == "manual" {
		res, err := p.TestConnection(ctx, platformp.TestConnectionRequest{})
		tryLog(res, err)
		return res, err
	}

	res, err := p.TestConnection(ctx, req)
	tryLog(res, err)
	return res, err
}
