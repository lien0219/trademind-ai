package inventory

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	platformp "github.com/trademind-ai/trademind/backend/internal/providers/platform"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

const inventoryRetryBatchMaxTasks = 100

func deriveInventorySyncBatchStatus(pending, running, success, failed int) string {
	if pending > 0 || running > 0 {
		return BatchStatusRunning
	}
	if failed == 0 {
		return BatchStatusSuccess
	}
	if success == 0 {
		return BatchStatusFailed
	}
	return BatchStatusPartialSuccess
}

func absInt(a int) int {
	if a < 0 {
		return -a
	}
	return a
}

func normalizeBatchSource(s string) string {
	return strings.TrimSpace(strings.ToLower(s))
}

func allowedInventoryBatchSource(s string) bool {
	switch normalizeBatchSource(s) {
	case BatchSourceManual, BatchSourceInventoryAlert, BatchSourceProductDetail, BatchSourceFailedRetry, BatchSourceOrderDeduct, BatchSourceSystem:
		return true
	default:
		return false
	}
}

func batchScopePresent(body CreateInventorySyncBatchBody) bool {
	if strings.TrimSpace(body.Platform) != "" {
		return true
	}
	if strings.TrimSpace(body.ShopID) != "" {
		return true
	}
	if strings.TrimSpace(body.ProductID) != "" {
		return true
	}
	if len(body.PublicationSkuIds) > 0 {
		return true
	}
	if len(body.ProductSkuIds) > 0 {
		return true
	}
	if body.OnlyAlerts {
		return true
	}
	return false
}

func effectiveAlertTypesForBatch(body CreateInventorySyncBatchBody) []string {
	if !body.OnlyAlerts {
		return nil
	}
	if len(body.AlertTypes) == 0 {
		return []string{AlertTypePlatformStockMismatch, AlertTypeInventorySyncFailed}
	}
	out := make([]string, 0, len(body.AlertTypes))
	for _, raw := range body.AlertTypes {
		t := strings.TrimSpace(strings.ToLower(raw))
		if t != "" {
			out = append(out, t)
		}
	}
	return out
}

func publicationSkuMatchesAlerts(localStock int, warningStock int, safetyStock int, platformStock *int,
	mismatchTh int, mismatchEnabled bool, latest latestTaskScan, alertTypes []string,
) bool {
	if len(alertTypes) == 0 {
		return true
	}
	for _, at := range alertTypes {
		switch strings.TrimSpace(strings.ToLower(at)) {
		case AlertTypeOutOfStock:
			if localStock <= 0 {
				return true
			}
		case AlertTypeLowStock:
			if localStock > 0 && (safetyStock == 0 || localStock > safetyStock) && localStock <= warningStock {
				return true
			}
		case AlertTypeBelowSafetyStock:
			if safetyStock > 0 && localStock > 0 && localStock <= safetyStock {
				return true
			}
		case AlertTypePlatformStockUnknown:
			if platformStock == nil {
				return true
			}
		case AlertTypePlatformStockMismatch:
			if mismatchEnabled && platformStock != nil && absInt(localStock-*platformStock) > mismatchTh {
				return true
			}
		case AlertTypeInventorySyncFailed:
			if strings.TrimSpace(latest.Status) == StatusFailed {
				return true
			}
		}
	}
	return false
}

func (s *Service) inventoryBatchMaxTasks(ctx context.Context) int {
	max := 500
	if s == nil || s.Settings == nil {
		return max
	}
	m, err := s.Settings.PlainByGroup(ctx, 0, "inventory")
	if err != nil {
		return max
	}
	if v := strings.TrimSpace(m["inventory_sync_batch_max_size"]); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 5000 {
			return n
		}
	}
	return max
}

func (s *Service) nextInventoryBatchNo(ctx context.Context) (string, error) {
	if s == nil || s.DB == nil {
		return "", fmt.Errorf("inventory: no db")
	}
	prefix := fmt.Sprintf("INV%s", time.Now().UTC().Format("20060102"))
	var last string
	err := s.DB.WithContext(ctx).Model(&InventorySyncBatch{}).
		Select("batch_no").
		Where("batch_no LIKE ?", prefix+"%").
		Order("batch_no DESC").
		Limit(1).
		Scan(&last).Error
	if err != nil || strings.TrimSpace(last) == "" {
		return prefix + fmt.Sprintf("%04d", 1), nil
	}
	suf := strings.TrimPrefix(strings.TrimSpace(last), prefix)
	n, err := strconv.Atoi(suf)
	if err != nil || n < 1 {
		return prefix + fmt.Sprintf("%04d", 1), nil
	}
	return prefix + fmt.Sprintf("%04d", n+1), nil
}

func appendSkippedReason(dst []string, reason string, limit int) []string {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return dst
	}
	for _, x := range dst {
		if x == reason {
			return dst
		}
	}
	if limit > 0 && len(dst) >= limit {
		return dst
	}
	return append(dst, reason)
}

func joinSkippedReason(lines []string, maxLen int) string {
	if len(lines) == 0 {
		return ""
	}
	s := strings.Join(lines, "; ")
	if maxLen > 0 && len(s) > maxLen {
		return s[:maxLen] + "…"
	}
	return s
}

type batchCandScan struct {
	PublicationSkuID  uuid.UUID  `gorm:"column:publication_sku_id"`
	PublicationID     uuid.UUID  `gorm:"column:publication_id"`
	ProductID         uuid.UUID  `gorm:"column:product_id"`
	ShopID            uuid.UUID  `gorm:"column:shop_id"`
	PlatformRaw       string     `gorm:"column:platform_raw"`
	ExternalProductID string     `gorm:"column:external_product_id"`
	ExternalSkuID     string     `gorm:"column:external_sku_id"`
	ProductSkuID      *uuid.UUID `gorm:"column:product_sku_id"`
	LocalStock        *int       `gorm:"column:local_stock"`
	PlatformStock     *int       `gorm:"column:platform_stock"`
	WarningStock      int        `gorm:"column:warning_stock"`
	SafetyStock       int        `gorm:"column:safety_stock"`
}

func (s *Service) queryInventoryBatchCandidates(ctx context.Context, body CreateInventorySyncBatchBody) ([]batchCandScan, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("inventory: no db")
	}
	tx := s.DB.WithContext(ctx).Table("product_publication_skus AS pps").
		Select(`pps.id AS publication_sku_id, pps.publication_id, pp.product_id, pp.shop_id, pp.platform AS platform_raw,
			pp.external_product_id, pps.external_sku_id, pps.product_sku_id, sk.stock AS local_stock,
			pps.stock AS platform_stock,
			sk.warning_stock AS warning_stock, sk.safety_stock AS safety_stock`).
		Joins("INNER JOIN product_publications pp ON pp.id = pps.publication_id AND pp.deleted_at IS NULL").
		Joins("LEFT JOIN product_skus sk ON sk.id = pps.product_sku_id AND sk.product_id = pp.product_id")

	if pid := strings.TrimSpace(body.ProductID); pid != "" {
		u, err := uuid.Parse(pid)
		if err != nil {
			return nil, fmt.Errorf("invalid productId")
		}
		tx = tx.Where("pp.product_id = ?", u)
	}
	if sid := strings.TrimSpace(body.ShopID); sid != "" {
		u, err := uuid.Parse(sid)
		if err != nil {
			return nil, fmt.Errorf("invalid shopId")
		}
		tx = tx.Where("pp.shop_id = ?", u)
	}
	if pl := strings.TrimSpace(strings.ToLower(body.Platform)); pl != "" {
		tx = tx.Where("LOWER(pp.platform) = ?", pl)
	}
	if body.effectiveOnlyPublished() {
		tx = tx.Where(`EXISTS (
			SELECT 1 FROM product_publications ppx WHERE ppx.id = pps.publication_id AND ppx.deleted_at IS NULL
		)`)
	}
	if len(body.ProductSkuIds) > 0 {
		uuids := make([]uuid.UUID, 0, len(body.ProductSkuIds))
		for _, raw := range body.ProductSkuIds {
			u, err := uuid.Parse(strings.TrimSpace(raw))
			if err != nil {
				continue
			}
			uuids = append(uuids, u)
		}
		if len(uuids) == 0 {
			return nil, fmt.Errorf("invalid productSkuIds")
		}
		tx = tx.Where("pps.product_sku_id IN ?", uuids)
	}
	if len(body.PublicationSkuIds) > 0 {
		uuids := make([]uuid.UUID, 0, len(body.PublicationSkuIds))
		for _, raw := range body.PublicationSkuIds {
			u, err := uuid.Parse(strings.TrimSpace(raw))
			if err != nil {
				continue
			}
			uuids = append(uuids, u)
		}
		if len(uuids) == 0 {
			return nil, fmt.Errorf("invalid publicationSkuIds")
		}
		tx = tx.Where("pps.id IN ?", uuids)
	}

	var rows []batchCandScan
	if err := tx.Order("pp.updated_at DESC, pps.created_at ASC").Scan(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func (s *Service) blockingPublicationSKUSet(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]struct{}, error) {
	out := map[uuid.UUID]struct{}{}
	if len(ids) == 0 {
		return out, nil
	}
	type row struct {
		P uuid.UUID `gorm:"column:publication_sku_id"`
	}
	var rows []row
	if err := s.DB.WithContext(ctx).Model(&InventorySyncTask{}).
		Select("DISTINCT publication_sku_id AS publication_sku_id").
		Where("publication_sku_id IN ? AND status IN ?", ids, []string{StatusPending, StatusRunning}).
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	for _, r := range rows {
		out[r.P] = struct{}{}
	}
	return out, nil
}

func trimBatchInputSummary(body CreateInventorySyncBatchBody) map[string]any {
	m := map[string]any{
		"source": normalizeBatchSource(body.Source),
	}
	if strings.TrimSpace(body.Platform) != "" {
		m["platform"] = strings.TrimSpace(strings.ToLower(body.Platform))
	}
	if strings.TrimSpace(body.ShopID) != "" {
		m["shopId"] = strings.TrimSpace(body.ShopID)
	}
	if strings.TrimSpace(body.ProductID) != "" {
		m["productId"] = strings.TrimSpace(body.ProductID)
	}
	if len(body.ProductSkuIds) > 0 {
		m["productSkuIdsCount"] = len(body.ProductSkuIds)
	}
	if len(body.PublicationSkuIds) > 0 {
		m["publicationSkuIdsCount"] = len(body.PublicationSkuIds)
	}
	m["onlyAlerts"] = body.OnlyAlerts
	if len(body.AlertTypes) > 0 {
		m["alertTypes"] = body.AlertTypes
	}
	m["onlyPublished"] = body.effectiveOnlyPublished()
	m["confirmAll"] = body.ConfirmAll
	m["force"] = body.Force
	return platformp.TrimRawMap(m, 24, 120)
}

func (s *Service) maybeReconcileInventoryBatch(ctx context.Context, bid *uuid.UUID) {
	if bid == nil || *bid == uuid.Nil {
		return
	}
	s.reconcileInventorySyncBatch(ctx, *bid)
}

func (s *Service) reconcileInventorySyncBatch(ctx context.Context, batchID uuid.UUID) {
	if s == nil || s.DB == nil {
		return
	}
	var prev InventorySyncBatch
	_ = s.DB.WithContext(ctx).First(&prev, "id = ?", batchID).Error

	var prevTerminal bool
	switch strings.TrimSpace(prev.Status) {
	case BatchStatusRunning, BatchStatusPending:
		prevTerminal = false
	default:
		prevTerminal = true
	}

	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var batch InventorySyncBatch
		if err := tx.First(&batch, "id = ?", batchID).Error; err != nil {
			return err
		}
		var rows []struct {
			Status string
			N      int64
		}
		if err := tx.Model(&InventorySyncTask{}).
			Select("status, COUNT(*) AS n").
			Where("batch_id = ?", batchID).
			Group("status").
			Scan(&rows).Error; err != nil {
			return err
		}
		var pend, run, succ, fail int64
		for _, r := range rows {
			switch strings.TrimSpace(r.Status) {
			case StatusPending:
				pend += r.N
			case StatusRunning:
				run += r.N
			case StatusSuccess:
				succ += r.N
			case StatusFailed:
				fail += r.N
			}
		}
		st := deriveInventorySyncBatchStatus(int(pend), int(run), int(succ), int(fail))
		now := time.Now().UTC()
		updates := map[string]any{
			"pending_count": int(pend),
			"running_count": int(run),
			"success_count": int(succ),
			"failed_count":  int(fail),
			"status":        st,
			"updated_at":    now,
		}
		if batch.StartedAt == nil && int(run+succ+fail) > 0 {
			updates["started_at"] = now
		}
		if st == BatchStatusRunning {
			updates["finished_at"] = nil
		} else if batch.FinishedAt == nil {
			tfin := now
			updates["finished_at"] = &tfin
		}
		sumOut := platformp.TrimRawMap(map[string]any{
			"pending": int(pend),
			"running": int(run),
			"success": int(succ),
			"failed":  int(fail),
			"status":  st,
		}, 16, 80)
		outB, _ := json.Marshal(sumOut)
		updates["output"] = datatypes.JSON(outB)

		return tx.Model(&InventorySyncBatch{}).Where("id = ?", batchID).Updates(updates).Error
	})
	if err != nil || s.OpLog == nil {
		return
	}

	var cur InventorySyncBatch
	if err := s.DB.WithContext(ctx).First(&cur, "id = ?", batchID).Error; err != nil {
		return
	}
	newTerminal := cur.Status != BatchStatusRunning && cur.Status != BatchStatusPending
	if newTerminal && !prevTerminal {
		action := "inventory.sync_batch.success"
		st := strings.TrimSpace(cur.Status)
		switch st {
		case BatchStatusPartialSuccess:
			action = "inventory.sync_batch.partial_success"
		case BatchStatusFailed:
			action = "inventory.sync_batch.failed"
		case BatchStatusSuccess:
			action = "inventory.sync_batch.success"
		default:
			action = "inventory.sync_batch.success"
		}
		msg := fmt.Sprintf("batchId=%s batchNo=%s status=%s created=%d skipped=%d ok=%d fail=%d",
			cur.ID.String(), cur.BatchNo, st, cur.TotalCount, cur.SkippedCount, cur.SuccessCount, cur.FailedCount)
		_ = s.OpLog.WriteBackground(ctx, operationlog.WriteOpts{
			AdminUserID: cur.CreatedBy,
			Action:      action,
			Resource:    "inventory_sync_batch",
			ResourceID:  cur.ID.String(),
			Status:      "success",
			Message:     clampStr(msg, 520),
		})
	}
}

func (s *Service) batchToDTO(ctx context.Context, b *InventorySyncBatch, shopLabel string, recent []TaskDTO) InventorySyncBatchDTO {
	var input any
	if len(b.Input) > 0 {
		_ = json.Unmarshal(b.Input, &input)
	}
	var output any
	if len(b.Output) > 0 {
		_ = json.Unmarshal(b.Output, &output)
	}
	pl := strings.TrimSpace(strings.ToLower(b.Platform))
	return InventorySyncBatchDTO{
		ID:            b.ID,
		BatchNo:       b.BatchNo,
		Source:        b.Source,
		Status:        b.Status,
		Platform:      pl,
		ShopID:        b.ShopID,
		ShopName:      shopLabel,
		ProductID:     b.ProductID,
		TotalCount:    b.TotalCount,
		PendingCount:  b.PendingCount,
		RunningCount:  b.RunningCount,
		SuccessCount:  b.SuccessCount,
		FailedCount:   b.FailedCount,
		SkippedCount:  b.SkippedCount,
		SkippedReason: b.SkippedReason,
		Input:         input,
		Output:        output,
		CreatedBy:     b.CreatedBy,
		StartedAt:     b.StartedAt,
		FinishedAt:    b.FinishedAt,
		CreatedAt:     b.CreatedAt,
		UpdatedAt:     b.UpdatedAt,
		RecentTasks:   recent,
	}
}

// CreateInventorySyncBatch creates batch rows + tasks (≤ configured max).
func (s *Service) CreateInventorySyncBatch(ctx context.Context, body CreateInventorySyncBatchBody, admin *uuid.UUID) (*InventorySyncBatchDTO, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("inventory: no db")
	}
	src := normalizeBatchSource(body.Source)
	if !allowedInventoryBatchSource(body.Source) {
		return nil, fmt.Errorf("invalid source")
	}
	body.Source = src

	if !body.ConfirmAll && !batchScopePresent(body) {
		return nil, fmt.Errorf("refused: empty scope; pass confirmAll=true for intentional broad sync")
	}

	maxTasks := s.inventoryBatchMaxTasks(ctx)
	pol, err := s.loadInventoryAlertPolicy(ctx)
	if err != nil {
		return nil, err
	}

	cands, err := s.queryInventoryBatchCandidates(ctx, body)
	if err != nil {
		return nil, err
	}

	alertTypes := effectiveAlertTypesForBatch(body)
	pubIDs := make([]uuid.UUID, 0, len(cands))
	for _, c := range cands {
		pubIDs = append(pubIDs, c.PublicationSkuID)
	}
	latestMap := s.loadLatestTasksByPubSku(ctx, pubIDs)

	type planned struct {
		scan batchCandScan
	}
	plannedRows := make([]planned, 0, len(cands))
	skipReasons := make([]string, 0, 8)

	for _, c := range cands {
		pl := strings.TrimSpace(strings.ToLower(c.PlatformRaw))
		localStock := derefStock(c.LocalStock)

		if body.OnlyAlerts {
			if !publicationSkuMatchesAlerts(localStock, c.WarningStock, c.SafetyStock, c.PlatformStock,
				pol.PlatformStockMismatchThresh, pol.AlertPlatformStockMismatch, latestMap[c.PublicationSkuID], alertTypes) {
				skipReasons = appendSkippedReason(skipReasons, "alert_filter_no_match", 12)
				continue
			}
		}

		if strings.TrimSpace(c.ExternalSkuID) == "" {
			skipReasons = appendSkippedReason(skipReasons, "missing_external_sku_id", 12)
			continue
		}
		extPID := strings.TrimSpace(c.ExternalProductID)
		if extPID == "" && pl != "amazon" {
			skipReasons = appendSkippedReason(skipReasons, "missing_external_product_id", 12)
			continue
		}
		if c.ProductSkuID == nil || *c.ProductSkuID == uuid.Nil {
			skipReasons = appendSkippedReason(skipReasons, "missing_product_sku_link", 12)
			continue
		}

		prov := platformp.Get(pl)
		if !platformp.IsInventorySyncRunnable(prov) {
			skipReasons = appendSkippedReason(skipReasons, fmt.Sprintf("platform_inventory_sync_not_runnable:%s", pl), 12)
			continue
		}

		shopRow, auth, err := s.Shops.PlainAuthForProviderCtx(ctx, c.ShopID)
		if err != nil {
			skipReasons = appendSkippedReason(skipReasons, "shop_auth_error", 12)
			continue
		}
		if err := ValidateShopInventoryPush(shopRow, auth, prov); err != nil {
			skipReasons = appendSkippedReason(skipReasons, fmt.Sprintf("shop_not_eligible:%s", pl), 12)
			continue
		}

		plannedRows = append(plannedRows, planned{scan: c})
	}

	blocking := map[uuid.UUID]struct{}{}
	if !body.Force && len(plannedRows) > 0 {
		ids := make([]uuid.UUID, 0, len(plannedRows))
		for _, p := range plannedRows {
			ids = append(ids, p.scan.PublicationSkuID)
		}
		blocking, err = s.blockingPublicationSKUSet(ctx, ids)
		if err != nil {
			return nil, err
		}
	}

	toCreate := make([]planned, 0, len(plannedRows))
	for _, p := range plannedRows {
		if _, blocked := blocking[p.scan.PublicationSkuID]; blocked && !body.Force {
			skipReasons = appendSkippedReason(skipReasons, "duplicate_pending_running_task", 12)
			continue
		}
		toCreate = append(toCreate, p)
	}

	if len(toCreate) > maxTasks {
		return nil, fmt.Errorf("too many inventory sync tasks in one batch (max %d)", maxTasks)
	}

	batchNo, err := s.nextInventoryBatchNo(ctx)
	if err != nil {
		return nil, err
	}

	inputSumm := trimBatchInputSummary(body)
	inputJSON, _ := json.Marshal(inputSumm)
	skippedN := len(cands) - len(toCreate)
	totalCount := len(cands)

	batchRow := InventorySyncBatch{
		BatchNo:       batchNo,
		Source:        body.Source,
		Status:        BatchStatusRunning,
		Platform:      strings.TrimSpace(strings.ToLower(body.Platform)),
		Input:         datatypes.JSON(inputJSON),
		SkippedReason: joinSkippedReason(skipReasons, 1800),
		TotalCount:    totalCount,
		PendingCount:  len(toCreate),
		RunningCount:  0,
		SuccessCount:  0,
		FailedCount:   0,
		SkippedCount:  skippedN,
		CreatedBy:     admin,
	}
	if pid := strings.TrimSpace(body.ProductID); pid != "" {
		u, err := uuid.Parse(pid)
		if err == nil {
			batchRow.ProductID = &u
		}
	}
	if sid := strings.TrimSpace(body.ShopID); sid != "" {
		u, err := uuid.Parse(sid)
		if err == nil {
			batchRow.ShopID = &u
		}
	}

	optCopy := platformp.TrimRawMap(body.Options, 12, 200)

	var createdTaskIDs []uuid.UUID
	err = s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&batchRow).Error; err != nil {
			return err
		}
		bid := batchRow.ID
		for _, p := range toCreate {
			c := p.scan
			pl := strings.TrimSpace(strings.ToLower(c.PlatformRaw))
			target := localStockFromScan(c)
			pubID := c.PublicationID
			pskuID := c.PublicationSkuID

			task := InventorySyncTask{
				BatchID:          &bid,
				BatchNo:          batchNo,
				ProductID:        c.ProductID,
				ProductSKUID:     c.ProductSkuID,
				PublicationID:    &pubID,
				PublicationSkuID: &pskuID,
				ShopID:           c.ShopID,
				Platform:         pl,
				TaskType:         TaskTypeInventorySync,
				Status:           StatusPending,
				Mode:             ModeBatch,
				TargetStock:      target,
				Input:            taskInputSnap(ModeBatch, target, pskuID, c.ProductSkuID, pubID, c.ShopID, optCopy, &bid, batchNo),
				CreatedBy:        admin,
			}
			if err := tx.Create(&task).Error; err != nil {
				return err
			}
			createdTaskIDs = append(createdTaskIDs, task.ID)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	for _, tid := range createdTaskIDs {
		if err := s.enqueueOrRunInventoryTask(ctx, tid); err != nil {
			return nil, err
		}
	}

	if s.OpLog != nil {
		plShort := strings.TrimSpace(strings.ToLower(body.Platform))
		shopShort := strings.TrimSpace(body.ShopID)
		msg := fmt.Sprintf("batchId=%s batchNo=%s source=%s created=%d skipped=%d platform=%s shopId=%s",
			batchRow.ID.String(), batchRow.BatchNo, batchRow.Source, len(toCreate), skippedN, plShort, shopShort)
		_ = s.OpLog.WriteBackground(ctx, operationlog.WriteOpts{
			AdminUserID: admin,
			Action:      "inventory.sync_batch.create",
			Resource:    "inventory_sync_batch",
			ResourceID:  batchRow.ID.String(),
			Status:      "success",
			Message:     clampStr(msg, 520),
		})
	}

	s.reconcileInventorySyncBatch(ctx, batchRow.ID)
	var refreshed InventorySyncBatch
	_ = s.DB.WithContext(ctx).First(&refreshed, "id = ?", batchRow.ID).Error
	shopLabel := ""
	if refreshed.ShopID != nil {
		shopLabel = s.shopNameLookup(ctx, *refreshed.ShopID)
	}
	out := s.batchToDTO(ctx, &refreshed, shopLabel, nil)
	return &out, nil
}

func localStockFromScan(c batchCandScan) int {
	return derefStock(c.LocalStock)
}

// ListInventorySyncBatches paginates batch rows for admin APIs.
func (s *Service) ListInventorySyncBatches(ctx context.Context, q InventorySyncBatchListQuery) (*InventorySyncBatchListResult, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("inventory: no db")
	}
	page := q.Page
	ps := q.PageSize
	if page < 1 {
		page = 1
	}
	if ps < 1 || ps > 100 {
		ps = 20
	}
	tx := s.DB.WithContext(ctx).Model(&InventorySyncBatch{})
	if strings.TrimSpace(q.Source) != "" {
		tx = tx.Where("source = ?", strings.TrimSpace(strings.ToLower(q.Source)))
	}
	if strings.TrimSpace(q.Status) != "" {
		tx = tx.Where("status = ?", strings.TrimSpace(strings.ToLower(q.Status)))
	}
	if strings.TrimSpace(q.Platform) != "" {
		tx = tx.Where("LOWER(platform) = ?", strings.TrimSpace(strings.ToLower(q.Platform)))
	}
	if q.ShopID != nil && *q.ShopID != uuid.Nil {
		tx = tx.Where("shop_id = ?", *q.ShopID)
	}
	if q.ProductID != nil && *q.ProductID != uuid.Nil {
		tx = tx.Where("product_id = ?", *q.ProductID)
	}
	if q.Start != nil {
		tx = tx.Where("created_at >= ?", *q.Start)
	}
	if q.End != nil {
		tx = tx.Where("created_at <= ?", *q.End)
	}
	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, err
	}
	offset := (page - 1) * ps
	var rows []InventorySyncBatch
	if err := tx.Order("created_at DESC").Offset(offset).Limit(ps).Find(&rows).Error; err != nil {
		return nil, err
	}
	items := make([]InventorySyncBatchDTO, 0, len(rows))
	for i := range rows {
		shopLabel := ""
		if rows[i].ShopID != nil {
			shopLabel = s.shopNameLookup(ctx, *rows[i].ShopID)
		}
		items = append(items, s.batchToDTO(ctx, &rows[i], shopLabel, nil))
	}
	return &InventorySyncBatchListResult{
		Items:      items,
		Total:      total,
		Page:       page,
		PageSize:   ps,
		TotalPages: pagesOf(total, ps),
	}, nil
}

// GetInventorySyncBatch returns one batch plus optional recent tasks (detail API).
func (s *Service) GetInventorySyncBatch(ctx context.Context, id uuid.UUID, recentLimit int) (*InventorySyncBatchDTO, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("inventory: no db")
	}
	var b InventorySyncBatch
	if err := s.DB.WithContext(ctx).First(&b, "id = ?", id).Error; err != nil {
		return nil, err
	}
	shopLabel := ""
	if b.ShopID != nil {
		shopLabel = s.shopNameLookup(ctx, *b.ShopID)
	}
	var recent []TaskDTO
	if recentLimit > 0 {
		var tasks []InventorySyncTask
		if err := s.DB.WithContext(ctx).Where("batch_id = ?", id).
			Order("created_at DESC").Limit(recentLimit).Find(&tasks).Error; err == nil {
			for _, t := range tasks {
				recent = append(recent, s.taskToDTO(ctx, &t, "", ""))
			}
		}
	}
	out := s.batchToDTO(ctx, &b, shopLabel, recent)
	return &out, nil
}

// ListInventorySyncBatchTasks lists tasks belonging to one batch (paginates via ListTasks).
func (s *Service) ListInventorySyncBatchTasks(ctx context.Context, batchID uuid.UUID, q ListQuery) (*ListTasksResult, error) {
	q.BatchID = &batchID
	return s.ListTasks(ctx, q)
}

// RetryInventorySyncBatchFailed retries every failed task still tied to this batch (same batch record).
func (s *Service) RetryInventorySyncBatchFailed(ctx context.Context, batchID uuid.UUID, admin *uuid.UUID) (*InventorySyncBatchDTO, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("inventory: no db")
	}
	var ids []uuid.UUID
	if err := s.DB.WithContext(ctx).Model(&InventorySyncTask{}).
		Where("batch_id = ? AND status = ?", batchID, StatusFailed).
		Pluck("id", &ids).Error; err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return nil, fmt.Errorf("no failed tasks in this batch")
	}
	for _, id := range ids {
		if _, err := s.RetryInventorySyncTask(ctx, id, admin); err != nil {
			return nil, err
		}
	}
	if s.OpLog != nil {
		msg := fmt.Sprintf("batchId=%s retriedFailed=%d", batchID.String(), len(ids))
		_ = s.OpLog.WriteBackground(ctx, operationlog.WriteOpts{
			AdminUserID: admin,
			Action:      "inventory.sync_batch.retry_failed",
			Resource:    "inventory_sync_batch",
			ResourceID:  batchID.String(),
			Status:      "success",
			Message:     clampStr(msg, 520),
		})
	}
	s.reconcileInventorySyncBatch(ctx, batchID)
	return s.GetInventorySyncBatch(ctx, batchID, 20)
}

// RetryInventorySyncTasksIntoBatch binds failed tasks to a new batch and retries them (≤100).
func (s *Service) RetryInventorySyncTasksIntoBatch(ctx context.Context, taskIDs []uuid.UUID, admin *uuid.UUID) (*InventorySyncBatchDTO, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("inventory: no db")
	}
	if len(taskIDs) == 0 {
		return nil, fmt.Errorf("taskIds required")
	}
	if len(taskIDs) > inventoryRetryBatchMaxTasks {
		return nil, fmt.Errorf("too many tasks (max %d)", inventoryRetryBatchMaxTasks)
	}
	var tasks []InventorySyncTask
	if err := s.DB.WithContext(ctx).Where("id IN ?", taskIDs).Find(&tasks).Error; err != nil {
		return nil, err
	}
	if len(tasks) != len(taskIDs) {
		return nil, fmt.Errorf("some tasks were not found")
	}
	for _, t := range tasks {
		if strings.TrimSpace(t.Status) != StatusFailed {
			return nil, fmt.Errorf("task %s is not failed", t.ID.String())
		}
	}
	batchNo, err := s.nextInventoryBatchNo(ctx)
	if err != nil {
		return nil, err
	}
	inputSumm := platformp.TrimRawMap(map[string]any{
		"source":       BatchSourceFailedRetry,
		"taskIdsCount": len(taskIDs),
	}, 24, 120)
	inputJSON, _ := json.Marshal(inputSumm)

	batchRow := InventorySyncBatch{
		BatchNo:      batchNo,
		Source:       BatchSourceFailedRetry,
		Status:       BatchStatusRunning,
		TotalCount:   len(taskIDs),
		PendingCount: len(taskIDs),
		Input:        datatypes.JSON(inputJSON),
		CreatedBy:    admin,
	}
	if err := s.DB.WithContext(ctx).Create(&batchRow).Error; err != nil {
		return nil, err
	}
	bid := batchRow.ID
	for _, t := range tasks {
		if err := s.DB.WithContext(ctx).Model(&InventorySyncTask{}).Where("id = ?", t.ID).
			Updates(map[string]any{
				"batch_id":   bid,
				"batch_no":   batchNo,
				"updated_at": time.Now().UTC(),
			}).Error; err != nil {
			return nil, err
		}
		if _, err := s.RetryInventorySyncTask(ctx, t.ID, admin); err != nil {
			return nil, err
		}
	}
	if s.OpLog != nil {
		msg := fmt.Sprintf("batchId=%s batchNo=%s retryTasks=%d", bid.String(), batchNo, len(taskIDs))
		_ = s.OpLog.WriteBackground(ctx, operationlog.WriteOpts{
			AdminUserID: admin,
			Action:      "inventory.sync_batch.retry_failed",
			Resource:    "inventory_sync_batch",
			ResourceID:  bid.String(),
			Status:      "success",
			Message:     clampStr(msg, 520),
		})
	}
	s.reconcileInventorySyncBatch(ctx, bid)
	return s.GetInventorySyncBatch(ctx, bid, 20)
}
