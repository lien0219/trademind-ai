package order

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/idempotency"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"gorm.io/datatypes"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	FulfillmentWaveDocumentPickList      = "pick_list"
	FulfillmentWaveDocumentPackingList   = "packing_list"
	FulfillmentWaveDocumentSKULabels     = "sku_labels"
	FulfillmentWaveDocumentPackageLabels = "package_labels"
)

var (
	ErrFulfillmentWaveDocumentNotFound              = errors.New("fulfillment wave document not found")
	ErrFulfillmentWaveDocumentReprintReasonRequired = errors.New("reprint reason is required")
)

type FulfillmentWaveDocumentSnapshot struct {
	WaveID         uuid.UUID                              `json:"waveId"`
	WaveNo         string                                 `json:"waveNo"`
	WaveStatus     string                                 `json:"waveStatus"`
	SourceRevision int                                    `json:"sourceRevision"`
	WarehouseID    uuid.UUID                              `json:"warehouseId"`
	WarehouseCode  string                                 `json:"warehouseCode,omitempty"`
	WarehouseName  string                                 `json:"warehouseName,omitempty"`
	GeneratedAt    time.Time                              `json:"generatedAt"`
	OrderCount     int                                    `json:"orderCount"`
	LineCount      int                                    `json:"lineCount"`
	RequiredQty    int                                    `json:"requiredQuantity"`
	PickedQty      int                                    `json:"pickedQuantity"`
	ShortageQty    int                                    `json:"shortageQuantity"`
	Orders         []FulfillmentWaveDocumentSnapshotOrder `json:"orders"`
	Lines          []FulfillmentWaveDocumentSnapshotLine  `json:"lines"`
}

type FulfillmentWaveDocumentSnapshotOrder struct {
	WaveOrderID       uuid.UUID `json:"waveOrderId"`
	OrderID           uuid.UUID `json:"orderId"`
	OrderNo           string    `json:"orderNo"`
	Status            string    `json:"status"`
	Carrier           string    `json:"carrier,omitempty"`
	TrackingNo        string    `json:"trackingNo,omitempty"`
	TrackingURL       string    `json:"trackingUrl,omitempty"`
	PackageCode       string    `json:"packageCode,omitempty"`
	ActualWeightGrams *int      `json:"actualWeightGrams,omitempty"`
}

type FulfillmentWaveDocumentSnapshotLine struct {
	WaveLineID   uuid.UUID `json:"waveLineId"`
	WaveOrderID  uuid.UUID `json:"waveOrderId"`
	OrderID      uuid.UUID `json:"orderId"`
	OrderNo      string    `json:"orderNo"`
	ProductTitle string    `json:"productTitle,omitempty"`
	SKUCode      string    `json:"skuCode,omitempty"`
	SKUName      string    `json:"skuName,omitempty"`
	Barcode      string    `json:"barcode,omitempty"`
	LocationCode string    `json:"locationCode,omitempty"`
	LocationName string    `json:"locationName,omitempty"`
	RequiredQty  int       `json:"requiredQuantity"`
	PickedQty    int       `json:"pickedQuantity"`
	ShortageQty  int       `json:"shortageQuantity"`
	Status       string    `json:"status"`
}

type GenerateFulfillmentWaveDocumentInput struct {
	ExpectedRevision int    `json:"expectedRevision"`
	IdempotencyKey   string `json:"idempotencyKey"`
}

type RecordFulfillmentWaveDocumentPrintInput struct {
	DocumentType   string `json:"documentType"`
	Copies         int    `json:"copies"`
	Reason         string `json:"reason,omitempty"`
	IdempotencyKey string `json:"idempotencyKey"`
}

type FulfillmentWaveDocumentSummary struct {
	ID             uuid.UUID  `json:"id"`
	WaveID         uuid.UUID  `json:"waveId"`
	Version        int        `json:"version"`
	SourceRevision int        `json:"sourceRevision"`
	SnapshotHash   string     `json:"snapshotHash"`
	CreatedBy      *uuid.UUID `json:"createdBy,omitempty"`
	CreatedAt      time.Time  `json:"createdAt"`
}

type FulfillmentWaveDocumentDetail struct {
	FulfillmentWaveDocumentSummary
	Snapshot    FulfillmentWaveDocumentSnapshot     `json:"snapshot"`
	PrintEvents []FulfillmentWaveDocumentPrintEvent `json:"printEvents"`
}

type FulfillmentWaveDocumentListResult struct {
	List       []FulfillmentWaveDocumentSummary `json:"list"`
	Pagination struct {
		Page       int   `json:"page"`
		PageSize   int   `json:"pageSize"`
		Total      int64 `json:"total"`
		TotalPages int   `json:"totalPages"`
	} `json:"pagination"`
}

func validFulfillmentWaveDocumentType(value string) bool {
	switch value {
	case FulfillmentWaveDocumentPickList, FulfillmentWaveDocumentPackingList,
		FulfillmentWaveDocumentSKULabels, FulfillmentWaveDocumentPackageLabels:
		return true
	default:
		return false
	}
}

func fulfillmentWaveDocumentSummary(row FulfillmentWaveDocument) FulfillmentWaveDocumentSummary {
	return FulfillmentWaveDocumentSummary{
		ID: row.ID, WaveID: row.WaveID, Version: row.Version, SourceRevision: row.SourceRevision,
		SnapshotHash: row.SnapshotHash, CreatedBy: row.CreatedBy, CreatedAt: row.CreatedAt,
	}
}

func loadFulfillmentWaveDocumentSnapshot(raw datatypes.JSON) (FulfillmentWaveDocumentSnapshot, error) {
	var snapshot FulfillmentWaveDocumentSnapshot
	if len(raw) == 0 || !json.Valid(raw) {
		return snapshot, errors.New("invalid fulfillment wave document snapshot")
	}
	if err := json.Unmarshal(raw, &snapshot); err != nil {
		return snapshot, fmt.Errorf("decode fulfillment wave document snapshot: %w", err)
	}
	return snapshot, nil
}

func buildFulfillmentWaveDocumentSnapshotTx(ctx context.Context, tx *gorm.DB, tenantID int64, wave *FulfillmentWave, generatedAt time.Time) (FulfillmentWaveDocumentSnapshot, error) {
	var orders []FulfillmentWaveOrder
	if err := tx.WithContext(ctx).Where("tenant_id = ? AND wave_id = ?", tenantID, wave.ID).Order("created_at ASC, id ASC").Find(&orders).Error; err != nil {
		return FulfillmentWaveDocumentSnapshot{}, err
	}
	var lines []FulfillmentWaveLine
	if err := tx.WithContext(ctx).Where("tenant_id = ? AND wave_id = ?", tenantID, wave.ID).Order("location_code ASC, sku_code ASC, order_id ASC, id ASC").Find(&lines).Error; err != nil {
		return FulfillmentWaveDocumentSnapshot{}, err
	}
	var warehouseLabel struct {
		Code string
		Name string
	}
	if err := tx.WithContext(ctx).Table("warehouses").Select("code, name").Where("tenant_id = ? AND id = ?", tenantID, wave.WarehouseID).Scan(&warehouseLabel).Error; err != nil {
		return FulfillmentWaveDocumentSnapshot{}, err
	}
	orderNos := make(map[uuid.UUID]string, len(orders))
	snapshotOrders := make([]FulfillmentWaveDocumentSnapshotOrder, 0, len(orders))
	for _, row := range orders {
		orderNos[row.OrderID] = row.OrderNo
		snapshotOrders = append(snapshotOrders, FulfillmentWaveDocumentSnapshotOrder{
			WaveOrderID: row.ID, OrderID: row.OrderID, OrderNo: row.OrderNo, Status: row.Status,
			Carrier: row.Carrier, TrackingNo: row.TrackingNo, TrackingURL: row.TrackingURL,
			PackageCode: row.PackageCode, ActualWeightGrams: row.ActualWeightGrams,
		})
	}
	snapshotLines := make([]FulfillmentWaveDocumentSnapshotLine, 0, len(lines))
	for _, row := range lines {
		snapshotLines = append(snapshotLines, FulfillmentWaveDocumentSnapshotLine{
			WaveLineID: row.ID, WaveOrderID: row.WaveOrderID, OrderID: row.OrderID, OrderNo: orderNos[row.OrderID],
			ProductTitle: row.ProductTitle, SKUCode: row.SKUCode, SKUName: row.SKUName, Barcode: row.Barcode,
			LocationCode: row.LocationCode, LocationName: row.LocationName, RequiredQty: row.RequiredQty,
			PickedQty: row.PickedQty, ShortageQty: row.ShortageQty, Status: row.Status,
		})
	}
	return FulfillmentWaveDocumentSnapshot{
		WaveID: wave.ID, WaveNo: wave.WaveNo, WaveStatus: wave.Status, SourceRevision: wave.Revision,
		WarehouseID: wave.WarehouseID, WarehouseCode: warehouseLabel.Code, WarehouseName: warehouseLabel.Name,
		GeneratedAt: generatedAt, OrderCount: wave.OrderCount, LineCount: wave.LineCount,
		RequiredQty: wave.RequiredQty, PickedQty: wave.PickedQty, ShortageQty: wave.ShortageQty,
		Orders: snapshotOrders, Lines: snapshotLines,
	}, nil
}

func (s *Service) GenerateFulfillmentWaveDocument(ctx context.Context, tenantID int64, principal *adminperm.Principal, actor *uuid.UUID, waveID uuid.UUID, in GenerateFulfillmentWaveDocumentInput) (*FulfillmentWaveDocumentDetail, error) {
	waveView, err := s.GetFulfillmentWave(ctx, tenantID, principal, waveID)
	if err != nil {
		return nil, err
	}
	if err := requireFulfillmentWaveOperate(principal, waveView); err != nil {
		return nil, err
	}
	key, err := normalizeWaveKey(in.IdempotencyKey)
	if err != nil || in.ExpectedRevision < 1 {
		return nil, ErrFulfillmentWaveInvalidInput
	}
	hash, err := fulfillmentWavePayloadHash(struct {
		Action   string    `json:"action"`
		WaveID   uuid.UUID `json:"waveId"`
		Revision int       `json:"revision"`
	}{"generate_document", waveID, in.ExpectedRevision})
	if err != nil {
		return nil, err
	}
	var documentID uuid.UUID
	created := false
	err = s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		wave, err := getFulfillmentWaveTx(ctx, tx, tenantID, waveID)
		if err != nil {
			return err
		}
		var existing FulfillmentWaveDocument
		existingErr := tx.Where("tenant_id = ? AND wave_id = ? AND idempotency_key = ?", tenantID, waveID, key).First(&existing).Error
		if existingErr == nil {
			if existing.RequestHash != hash {
				return ErrFulfillmentWaveIdempotency
			}
			documentID = existing.ID
			return nil
		}
		if !errors.Is(existingErr, gorm.ErrRecordNotFound) {
			return existingErr
		}
		if wave.Revision != in.ExpectedRevision {
			return ErrFulfillmentWaveRevision
		}
		if wave.Status == FulfillmentWaveCancelled {
			return ErrFulfillmentWaveState
		}
		var latestVersion int
		if err := tx.Model(&FulfillmentWaveDocument{}).Select("COALESCE(MAX(version), 0)").Where("tenant_id = ? AND wave_id = ?", tenantID, waveID).Scan(&latestVersion).Error; err != nil {
			return err
		}
		now := time.Now().UTC()
		snapshot, err := buildFulfillmentWaveDocumentSnapshotTx(ctx, tx, tenantID, wave, now)
		if err != nil {
			return err
		}
		raw, err := json.Marshal(snapshot)
		if err != nil {
			return fmt.Errorf("encode fulfillment wave document snapshot: %w", err)
		}
		row := &FulfillmentWaveDocument{
			TenantID: tenantID, WaveID: waveID, Version: latestVersion + 1, SourceRevision: wave.Revision,
			IdempotencyKey: key, RequestHash: hash, SnapshotHash: idempotency.HashRequest(raw),
			Snapshot: datatypes.JSON(raw), CreatedBy: actor,
		}
		if err := tx.Create(row).Error; err != nil {
			if isFulfillmentWaveUniqueViolation(err) {
				return ErrFulfillmentWaveIdempotency
			}
			return err
		}
		documentID = row.ID
		created = true
		return nil
	})
	if err != nil {
		return nil, err
	}
	if created {
		s.writeFulfillmentWaveLog(ctx, tenantID, actor, waveID, "document.generate", "documentId="+documentID.String())
	}
	return s.GetFulfillmentWaveDocument(ctx, tenantID, principal, waveID, documentID)
}

func (s *Service) ListFulfillmentWaveDocuments(ctx context.Context, tenantID int64, principal *adminperm.Principal, waveID uuid.UUID, page, pageSize int) (*FulfillmentWaveDocumentListResult, error) {
	if _, err := s.GetFulfillmentWave(ctx, tenantID, principal, waveID); err != nil {
		return nil, err
	}
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	q := s.DB.WithContext(ctx).Model(&FulfillmentWaveDocument{}).Where("tenant_id = ? AND wave_id = ?", tenantID, waveID)
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, err
	}
	var rows []FulfillmentWaveDocument
	if err := q.Order("version DESC, id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&rows).Error; err != nil {
		return nil, err
	}
	list := make([]FulfillmentWaveDocumentSummary, 0, len(rows))
	for _, row := range rows {
		list = append(list, fulfillmentWaveDocumentSummary(row))
	}
	out := &FulfillmentWaveDocumentListResult{List: list}
	out.Pagination.Page = page
	out.Pagination.PageSize = pageSize
	out.Pagination.Total = total
	out.Pagination.TotalPages = int((total + int64(pageSize) - 1) / int64(pageSize))
	return out, nil
}

func (s *Service) GetFulfillmentWaveDocument(ctx context.Context, tenantID int64, principal *adminperm.Principal, waveID, documentID uuid.UUID) (*FulfillmentWaveDocumentDetail, error) {
	if _, err := s.GetFulfillmentWave(ctx, tenantID, principal, waveID); err != nil {
		return nil, err
	}
	var row FulfillmentWaveDocument
	err := s.DB.WithContext(ctx).Where("tenant_id = ? AND wave_id = ? AND id = ?", tenantID, waveID, documentID).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrFulfillmentWaveDocumentNotFound
	}
	if err != nil {
		return nil, err
	}
	snapshot, err := loadFulfillmentWaveDocumentSnapshot(row.Snapshot)
	if err != nil {
		return nil, err
	}
	var events []FulfillmentWaveDocumentPrintEvent
	if err := s.DB.WithContext(ctx).Where("tenant_id = ? AND wave_id = ? AND document_id = ?", tenantID, waveID, documentID).Order("created_at DESC, id DESC").Find(&events).Error; err != nil {
		return nil, err
	}
	return &FulfillmentWaveDocumentDetail{
		FulfillmentWaveDocumentSummary: fulfillmentWaveDocumentSummary(row),
		Snapshot:                       snapshot, PrintEvents: events,
	}, nil
}

func (s *Service) RecordFulfillmentWaveDocumentPrint(ctx context.Context, tenantID int64, principal *adminperm.Principal, actor *uuid.UUID, waveID, documentID uuid.UUID, in RecordFulfillmentWaveDocumentPrintInput) (*FulfillmentWaveDocumentPrintEvent, error) {
	wave, err := s.GetFulfillmentWave(ctx, tenantID, principal, waveID)
	if err != nil {
		return nil, err
	}
	if err := requireFulfillmentWaveOperate(principal, wave); err != nil {
		return nil, err
	}
	if wave.Status == FulfillmentWaveCancelled {
		return nil, ErrFulfillmentWaveState
	}
	key, err := normalizeWaveKey(in.IdempotencyKey)
	documentType := strings.TrimSpace(in.DocumentType)
	reason := clampWaveText(in.Reason, 500)
	if err != nil || !validFulfillmentWaveDocumentType(documentType) || in.Copies < 1 || in.Copies > 100 {
		return nil, ErrFulfillmentWaveInvalidInput
	}
	hash, err := fulfillmentWavePayloadHash(struct {
		Action       string    `json:"action"`
		DocumentID   uuid.UUID `json:"documentId"`
		DocumentType string    `json:"documentType"`
		Copies       int       `json:"copies"`
		Reason       string    `json:"reason"`
	}{"record_print", documentID, documentType, in.Copies, reason})
	if err != nil {
		return nil, err
	}
	var event FulfillmentWaveDocumentPrintEvent
	created := false
	err = s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var document FulfillmentWaveDocument
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("tenant_id = ? AND wave_id = ? AND id = ?", tenantID, waveID, documentID).First(&document).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrFulfillmentWaveDocumentNotFound
		}
		if err != nil {
			return err
		}
		var existing FulfillmentWaveDocumentPrintEvent
		existingErr := tx.Where("tenant_id = ? AND document_id = ? AND idempotency_key = ?", tenantID, documentID, key).First(&existing).Error
		if existingErr == nil {
			if existing.RequestHash != hash {
				return ErrFulfillmentWaveIdempotency
			}
			event = existing
			return nil
		}
		if !errors.Is(existingErr, gorm.ErrRecordNotFound) {
			return existingErr
		}
		var prior int64
		if err := tx.Model(&FulfillmentWaveDocumentPrintEvent{}).Where("tenant_id = ? AND document_id = ? AND document_type = ?", tenantID, documentID, documentType).Count(&prior).Error; err != nil {
			return err
		}
		if prior > 0 && len([]rune(reason)) < 2 {
			return ErrFulfillmentWaveDocumentReprintReasonRequired
		}
		event = FulfillmentWaveDocumentPrintEvent{
			TenantID: tenantID, WaveID: waveID, DocumentID: documentID, DocumentType: documentType,
			Copies: in.Copies, Reprint: prior > 0, Reason: reason, IdempotencyKey: key, RequestHash: hash, ActorID: actor,
		}
		if err := tx.Create(&event).Error; err != nil {
			if isFulfillmentWaveUniqueViolation(err) {
				return ErrFulfillmentWaveIdempotency
			}
			return err
		}
		created = true
		return nil
	})
	if err != nil {
		return nil, err
	}
	if created {
		action := "document.print"
		if event.Reprint {
			action = "document.reprint"
		}
		s.writeFulfillmentWaveLog(ctx, tenantID, actor, waveID, action, fmt.Sprintf("documentId=%s type=%s copies=%d", documentID, documentType, in.Copies))
	}
	return &event, nil
}
