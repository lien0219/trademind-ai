package product

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/files"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	platformdouyin "github.com/trademind-ai/trademind/backend/internal/providers/platform/douyinshop"
	"github.com/trademind-ai/trademind/backend/internal/providers/storage"
	"golang.org/x/image/webp"
	"gorm.io/datatypes"
	"gorm.io/gorm/clause"
)

const (
	DouyinImageStatusPending    = "pending"
	DouyinImageStatusProcessing = "processing"
	DouyinImageStatusUploaded   = "uploaded"
	DouyinImageStatusFailed     = "failed"
	DouyinImageStatusSkipped    = "skipped"

	ImageURLNotAccessible   = "IMAGE_URL_NOT_ACCESSIBLE"
	ImageDownloadFailed     = "IMAGE_DOWNLOAD_FAILED"
	ImageReadFailed         = "IMAGE_READ_FAILED"
	ImageFormatUnsupported  = "IMAGE_FORMAT_UNSUPPORTED"
	ImageSizeTooLarge       = "IMAGE_SIZE_TOO_LARGE"
	ImageDimensionInvalid   = "IMAGE_DIMENSION_INVALID"
	ImageProcessFailed      = "IMAGE_PROCESS_FAILED"
	StorageUploadFailed     = "STORAGE_UPLOAD_FAILED"
	DouyinImageUploadFailed = "DOUYIN_IMAGE_UPLOAD_FAILED"
)

const douyinImageMaxBytes = 10 << 20

type DouyinImageUploadBody struct {
	ImageTypes  []string `json:"imageTypes"`
	RetryFailed bool     `json:"retryFailed"`
	Force       bool     `json:"force"`
}

type DouyinImageUploadResult struct {
	ProductID string              `json:"productId"`
	Platform  string              `json:"platform"`
	Summary   DouyinImageSummary  `json:"summary"`
	Mapping   *DouyinDraftMapping `json:"mapping"`
}

type DouyinImageSummary struct {
	Uploaded int `json:"uploaded"`
	Skipped  int `json:"skipped"`
	Failed   int `json:"failed"`
	Pending  int `json:"pending"`
}

type imageUploadSource struct {
	Data       []byte
	FileName   string
	MimeType   string
	StorageURL string
	StorageKey string
	SourceURL  string
	Processed  bool
}

func (s *Service) UploadDouyinImages(c *gin.Context, productID uuid.UUID, body DouyinImageUploadBody, adminID *uuid.UUID, filesSvc *files.Service) (*DouyinImageUploadResult, error) {
	return s.uploadDouyinImages(c, productID, body, "", adminID, filesSvc)
}

func (s *Service) RetryDouyinImage(c *gin.Context, productID uuid.UUID, imageKey string, adminID *uuid.UUID, filesSvc *files.Service) (*DouyinImageUploadResult, error) {
	body := DouyinImageUploadBody{ImageTypes: []string{"main", "detail"}, RetryFailed: true, Force: true}
	return s.uploadDouyinImages(c, productID, body, strings.TrimSpace(imageKey), adminID, filesSvc)
}

func (s *Service) GetDouyinImageStatus(ctx context.Context, productID uuid.UUID) (*DouyinImageUploadResult, error) {
	m, err := s.GetDouyinDraftMapping(ctx, productID)
	if err != nil {
		return nil, err
	}
	return &DouyinImageUploadResult{
		ProductID: productID.String(),
		Platform:  "douyin_shop",
		Summary:   summarizeDouyinImages(m),
		Mapping:   m,
	}, nil
}

func (s *Service) uploadDouyinImages(c *gin.Context, productID uuid.UUID, body DouyinImageUploadBody, onlyKey string, adminID *uuid.UUID, filesSvc *files.Service) (*DouyinImageUploadResult, error) {
	if s == nil || s.DB == nil || filesSvc == nil {
		return nil, fmt.Errorf("product: misconfigured")
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 120*time.Second)
	defer cancel()
	m, err := s.GetDouyinDraftMapping(ctx, productID)
	if err != nil {
		return nil, err
	}
	shopID, err := uuid.Parse(strings.TrimSpace(m.ShopID))
	if err != nil || shopID == uuid.Nil {
		return nil, fmt.Errorf("douyin shopId required")
	}
	uploader, err := s.douyinUploaderForShop(c, ctx, shopID, adminID)
	if err != nil {
		return nil, err
	}
	types := normalizeDouyinImageTypes(body.ImageTypes)
	logDouyinProductImage(c, s, adminID, productID, "douyin.image.upload.start", "success", "", fmt.Sprintf("types=%s retryFailed=%v", strings.Join(types, ","), body.RetryFailed))

	run := func(items []DouyinDraftImage, typ string) []DouyinDraftImage {
		for i := range items {
			if !shouldUploadDouyinImage(items[i], typ, types, body, onlyKey, i) {
				if strings.TrimSpace(items[i].UploadStatus) == "" {
					items[i].UploadStatus = DouyinImageStatusSkipped
				}
				continue
			}
			items[i].UploadStatus = DouyinImageStatusProcessing
			items[i].ErrorCode = ""
			items[i].ErrorMessage = ""
			src, srcErr := s.resolveDouyinImageSource(ctx, productID, &items[i], adminID, filesSvc)
			if srcErr != nil {
				markDouyinImageFailed(&items[i], imageErrCode(srcErr), safeImageErr(srcErr))
				continue
			}
			platformImage, upErr := uploader.UploadImage(ctx, shopID.String(), platformdouyin.UploadImageRequest{
				ImageType: typ,
				FileName:  src.FileName,
				MimeType:  src.MimeType,
				Reader:    bytes.NewReader(src.Data),
				SourceURL: src.StorageURL,
			})
			if upErr != nil {
				markDouyinImageFailed(&items[i], providerImageErrCode(upErr), safeImageErr(upErr))
				continue
			}
			now := time.Now().UTC()
			items[i].PlatformImageID = strings.TrimSpace(platformImage.ImageID)
			items[i].PlatformImageURL = strings.TrimSpace(platformImage.URL)
			items[i].StorageURL = strings.TrimSpace(src.StorageURL)
			items[i].StorageKey = strings.TrimSpace(src.StorageKey)
			items[i].PublicURL = firstNonEmpty(items[i].PublicURL, src.StorageURL)
			items[i].ObjectKey = firstNonEmpty(items[i].ObjectKey, src.StorageKey)
			items[i].Processed = items[i].Processed || src.Processed
			items[i].UploadStatus = DouyinImageStatusUploaded
			items[i].Status = "uploaded"
			items[i].NeedSync = false
			items[i].UploadedAt = &now
			items[i].Raw = platformImage.Raw
			logDouyinProductImage(c, s, adminID, productID, "douyin.image.upload.success", "success", "", "image uploaded")
		}
		return items
	}
	m.MainImages = run(m.MainImages, ImageTypeMain)
	m.DetailImages = run(m.DetailImages, ImageTypeDetail)
	ApplyDouyinDraftValidation(m, s.douyinPricingProtection(ctx))
	if err := s.saveDouyinDraftMappingNoValidation(ctx, productID, m); err != nil {
		return nil, err
	}
	out := &DouyinImageUploadResult{ProductID: productID.String(), Platform: "douyin_shop", Summary: summarizeDouyinImages(m), Mapping: m}
	if out.Summary.Failed > 0 {
		logDouyinProductImage(c, s, adminID, productID, "douyin.image.upload.failed", "failed", DouyinImageUploadFailed, fmt.Sprintf("failed=%d", out.Summary.Failed))
	}
	if onlyKey != "" {
		logDouyinProductImage(c, s, adminID, productID, "douyin.image.upload.retry", "success", "", "imageKey="+onlyKey)
	}
	return out, nil
}

func (s *Service) douyinUploaderForShop(c *gin.Context, ctx context.Context, shopID uuid.UUID, adminID *uuid.UUID) (DouyinImageUploader, error) {
	if s.DouyinImageUploader != nil {
		return s.DouyinImageUploader, nil
	}
	if s.Shops == nil {
		return nil, fmt.Errorf("douyin shop client unavailable")
	}
	client, _, err := s.Shops.DouyinClientForShop(c, ctx, shopID, adminID)
	if err != nil {
		return nil, err
	}
	return client, nil
}

func (s *Service) resolveDouyinImageSource(ctx context.Context, productID uuid.UUID, img *DouyinDraftImage, adminID *uuid.UUID, filesSvc *files.Service) (*imageUploadSource, error) {
	key := firstNonEmpty(img.StorageKey, img.ObjectKey)
	if key != "" {
		return s.readStorageDouyinImage(ctx, key, img)
	}
	rawURL := firstNonEmpty(img.SourceURL, img.OriginURL, img.URL, img.PublicURL)
	if rawURL == "" || strings.HasPrefix(strings.ToLower(strings.TrimSpace(rawURL)), "data:") {
		return nil, imageCodeErr{code: ImageURLNotAccessible, msg: "image url is empty or base64 data"}
	}
	data, err := fetchDouyinRemoteImage(ctx, rawURL)
	if err != nil {
		return nil, err
	}
	ext, mimeType, err := validateDouyinImageBytes(data, rawURL)
	if err != nil {
		return nil, err
	}
	objKey := fmt.Sprintf("%s/douyin-sync-%s%s", time.Now().UTC().Format("2006/01/02"), randHex(8), ext)
	rec, err := filesSvc.SaveProcessed(ctx, files.SaveProcessedOpts{
		OriginalName: filepath.Base(strings.Split(rawURL, "?")[0]),
		ObjectKey:    objKey,
		Data:         data,
		ContentType:  mimeType,
		CreatedBy:    adminID,
	})
	if err != nil {
		return nil, imageCodeErr{code: StorageUploadFailed, msg: err.Error()}
	}
	img.SourceURL = rawURL
	img.StorageURL = rec.PublicURL
	img.StorageKey = rec.ObjectKey
	img.PublicURL = rec.PublicURL
	img.ObjectKey = rec.ObjectKey
	img.NeedSync = false
	_ = s.updateProductImageStorage(ctx, productID, img.LocalImageID, rec.ObjectKey, rec.PublicURL, rawURL)
	logDouyinProductImage(nil, s, adminID, productID, "douyin.image.storage.sync", "success", "", "external image synced to storage")
	return &imageUploadSource{Data: data, FileName: rec.OriginalName, MimeType: mimeType, StorageURL: rec.PublicURL, StorageKey: rec.ObjectKey, SourceURL: rawURL}, nil
}

func (s *Service) readStorageDouyinImage(ctx context.Context, key string, img *DouyinDraftImage) (*imageUploadSource, error) {
	plain, err := s.Settings.PlainByGroup(ctx, 0, "storage")
	if err != nil {
		return nil, imageCodeErr{code: ImageReadFailed, msg: err.Error()}
	}
	prov, _, err := storage.NewFromPlain(plain)
	if err != nil {
		return nil, imageCodeErr{code: ImageReadFailed, msg: err.Error()}
	}
	rc, err := prov.Get(ctx, key)
	if err != nil {
		return nil, imageCodeErr{code: ImageReadFailed, msg: err.Error()}
	}
	defer rc.Close()
	data, err := io.ReadAll(io.LimitReader(rc, douyinImageMaxBytes+1))
	if err != nil {
		return nil, imageCodeErr{code: ImageReadFailed, msg: err.Error()}
	}
	_, mimeType, err := validateDouyinImageBytes(data, key)
	if err != nil {
		return nil, err
	}
	pubURL := strings.TrimSpace(img.StorageURL)
	if pubURL == "" {
		pubURL, _ = prov.GetURL(ctx, key)
	}
	return &imageUploadSource{Data: data, FileName: filepath.Base(key), MimeType: mimeType, StorageURL: pubURL, StorageKey: key, SourceURL: firstNonEmpty(img.SourceURL, img.OriginURL), Processed: false}, nil
}

func fetchDouyinRemoteImage(ctx context.Context, rawURL string) ([]byte, error) {
	if err := validateExternalImageURL(rawURL); err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, imageCodeErr{code: ImageURLNotAccessible, msg: err.Error()}
	}
	req.Header.Set("User-Agent", "TradeMind-DouyinImageUpload/1.0")
	req.Header.Set("Accept", "image/*")
	resp, err := (&http.Client{Timeout: 30 * time.Second}).Do(req)
	if err != nil {
		return nil, imageCodeErr{code: ImageDownloadFailed, msg: err.Error()}
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, imageCodeErr{code: ImageURLNotAccessible, msg: fmt.Sprintf("http %d", resp.StatusCode)}
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, douyinImageMaxBytes+1))
	if err != nil {
		return nil, imageCodeErr{code: ImageDownloadFailed, msg: err.Error()}
	}
	return data, nil
}

func validateExternalImageURL(raw string) error {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Hostname() == "" {
		return imageCodeErr{code: ImageURLNotAccessible, msg: "invalid image url"}
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return imageCodeErr{code: ImageURLNotAccessible, msg: "only http/https image urls are allowed"}
	}
	ips, err := net.LookupIP(u.Hostname())
	if err != nil || len(ips) == 0 {
		return imageCodeErr{code: ImageURLNotAccessible, msg: "image host is not resolvable"}
	}
	for _, ip := range ips {
		if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified() {
			return imageCodeErr{code: ImageURLNotAccessible, msg: "private network image urls are not allowed"}
		}
	}
	return nil
}

func validateDouyinImageBytes(data []byte, name string) (string, string, error) {
	if len(data) == 0 {
		return "", "", imageCodeErr{code: ImageReadFailed, msg: "empty image"}
	}
	if len(data) > douyinImageMaxBytes {
		return "", "", imageCodeErr{code: ImageSizeTooLarge, msg: "image exceeds douyin 10MB limit"}
	}
	ct := http.DetectContentType(data)
	ext := strings.ToLower(filepath.Ext(strings.Split(name, "?")[0]))
	mimeType := ""
	switch {
	case strings.Contains(ct, "jpeg") || ext == ".jpg" || ext == ".jpeg":
		ext, mimeType = ".jpg", "image/jpeg"
	case strings.Contains(ct, "png") || ext == ".png":
		ext, mimeType = ".png", "image/png"
	case strings.Contains(ct, "webp") || ext == ".webp":
		ext, mimeType = ".webp", "image/webp"
	default:
		return "", "", imageCodeErr{code: ImageFormatUnsupported, msg: "only jpg, png and webp images are supported"}
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil && mimeType == "image/webp" {
		cfg, err = webp.DecodeConfig(bytes.NewReader(data))
	}
	if err != nil {
		return "", "", imageCodeErr{code: ImageDimensionInvalid, msg: err.Error()}
	}
	if cfg.Width <= 0 || cfg.Height <= 0 {
		return "", "", imageCodeErr{code: ImageDimensionInvalid, msg: "invalid image dimensions"}
	}
	return ext, mimeType, nil
}

func (s *Service) saveDouyinDraftMappingNoValidation(ctx context.Context, productID uuid.UUID, mapping *DouyinDraftMapping) error {
	imagesJSON, err := json.Marshal(map[string]any{"mainImages": mapping.MainImages, "detailImages": mapping.DetailImages})
	if err != nil {
		return err
	}
	warnJSON, _ := json.Marshal(mapping.Warnings)
	errJSON, _ := json.Marshal(mapping.Errors)
	now := time.Now().UTC()
	row := ProductPlatformPublishConfig{
		ProductID:       productID,
		Platform:        "douyin_shop",
		MappedImages:    datatypes.JSON(imagesJSON),
		MappingWarnings: datatypes.JSON(warnJSON),
		MappingErrors:   datatypes.JSON(errJSON),
		LastMappedAt:    mapping.LastMappedAt,
	}
	if row.LastMappedAt == nil {
		row.LastMappedAt = &now
	}
	return s.DB.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "product_id"}, {Name: "platform"}},
		DoUpdates: clause.AssignmentColumns([]string{"mapped_images", "mapping_warnings", "mapping_errors", "last_mapped_at", "updated_at"}),
	}).Create(&row).Error
}

func (s *Service) updateProductImageStorage(ctx context.Context, productID uuid.UUID, localImageID, objectKey, publicURL, originURL string) error {
	u, err := uuid.Parse(strings.TrimSpace(localImageID))
	if err != nil || u == uuid.Nil {
		return nil
	}
	return s.DB.WithContext(ctx).Model(&ProductImage{}).Where("id = ? AND product_id = ?", u, productID).Updates(map[string]any{
		"object_key":  objectKey,
		"storage_key": objectKey,
		"public_url":  publicURL,
		"origin_url":  originURL,
		"source":      "sync",
	}).Error
}

func shouldUploadDouyinImage(img DouyinDraftImage, typ string, types []string, body DouyinImageUploadBody, onlyKey string, idx int) bool {
	if !containsImageType(types, typ) {
		return false
	}
	if onlyKey != "" && !imageKeyMatches(img, typ, onlyKey, idx) {
		return false
	}
	if !body.Force && strings.TrimSpace(img.PlatformImageID) != "" && strings.EqualFold(img.UploadStatus, DouyinImageStatusUploaded) {
		return false
	}
	if strings.EqualFold(img.UploadStatus, DouyinImageStatusFailed) {
		return body.RetryFailed || body.Force || onlyKey != ""
	}
	return true
}

func imageKeyMatches(img DouyinDraftImage, typ, key string, idx int) bool {
	key = strings.TrimSpace(key)
	return key == strings.TrimSpace(img.LocalImageID) ||
		key == fmt.Sprintf("%s:%d", typ, idx) ||
		key == strings.TrimSpace(img.StorageKey) ||
		key == strings.TrimSpace(img.PlatformImageID)
}

func normalizeDouyinImageTypes(in []string) []string {
	if len(in) == 0 {
		return []string{ImageTypeMain, ImageTypeDetail}
	}
	out := make([]string, 0, 2)
	for _, v := range in {
		t := strings.ToLower(strings.TrimSpace(v))
		if t == ImageTypeDescription {
			t = ImageTypeDetail
		}
		if (t == ImageTypeMain || t == ImageTypeDetail) && !containsImageType(out, t) {
			out = append(out, t)
		}
	}
	if len(out) == 0 {
		return []string{ImageTypeMain, ImageTypeDetail}
	}
	return out
}

func containsImageType(items []string, typ string) bool {
	for _, v := range items {
		if v == typ {
			return true
		}
	}
	return false
}

func markDouyinImageFailed(img *DouyinDraftImage, code, msg string) {
	img.UploadStatus = DouyinImageStatusFailed
	img.Status = DouyinImageStatusFailed
	img.ErrorCode = strings.TrimSpace(code)
	img.ErrorMessage = safeImageErr(errors.New(msg))
	if img.ErrorCode == "" {
		img.ErrorCode = DouyinImageUploadFailed
	}
}

func summarizeDouyinImages(m *DouyinDraftMapping) DouyinImageSummary {
	var out DouyinImageSummary
	for _, img := range append(append([]DouyinDraftImage{}, m.MainImages...), m.DetailImages...) {
		switch strings.ToLower(strings.TrimSpace(img.UploadStatus)) {
		case DouyinImageStatusUploaded:
			out.Uploaded++
		case DouyinImageStatusFailed:
			out.Failed++
		case DouyinImageStatusSkipped:
			out.Skipped++
		default:
			out.Pending++
		}
	}
	return out
}

type imageCodeErr struct {
	code string
	msg  string
}

func (e imageCodeErr) Error() string { return strings.TrimSpace(e.msg) }

func imageErrCode(err error) string {
	var ice imageCodeErr
	if errors.As(err, &ice) {
		return ice.code
	}
	return ImageProcessFailed
}

func providerImageErrCode(err error) string {
	var de *platformdouyin.Error
	if errors.As(err, &de) {
		return de.Code
	}
	return DouyinImageUploadFailed
}

func safeImageErr(err error) string {
	if err == nil {
		return ""
	}
	msg := err.Error()
	for _, marker := range []string{"access_token", "refresh_token", "app_secret", "secret", "token"} {
		if strings.Contains(strings.ToLower(msg), marker) {
			return "image upload failed"
		}
	}
	if len(msg) > 300 {
		msg = msg[:300] + "..."
	}
	return msg
}

func randHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return uuid.NewString()
	}
	return hex.EncodeToString(b)
}

func logDouyinProductImage(c *gin.Context, s *Service, adminID *uuid.UUID, productID uuid.UUID, action, status, code, msg string) {
	if s == nil || s.OpLog == nil {
		return
	}
	if strings.TrimSpace(code) != "" {
		msg = "code=" + code + " " + msg
	}
	opts := operationlog.WriteOpts{AdminUserID: adminID, Action: action, Resource: "product", ResourceID: productID.String(), Status: status, Message: msg}
	if c != nil {
		_ = s.OpLog.Write(c, opts)
		return
	}
	_ = s.OpLog.WriteBackground(context.Background(), opts)
}
