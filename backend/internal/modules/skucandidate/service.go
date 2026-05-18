package skucandidate

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/order"
	"github.com/trademind-ai/trademind/backend/internal/modules/product"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// Service computes read-only SKU suggestions.
type Service struct {
	DB *gorm.DB
}

type candidateAcc struct {
	skuID      uuid.UUID
	productID  uuid.UUID
	scoreParts map[string]int
	signals    []string
}

func (s *Service) accumulate(a map[uuid.UUID]*candidateAcc, id uuid.UUID, pid uuid.UUID) *candidateAcc {
	row, ok := a[id]
	if !ok {
		row = &candidateAcc{
			skuID:      id,
			productID:  pid,
			scoreParts: map[string]int{},
		}
		a[id] = row
	}
	return row
}

func (acc *candidateAcc) addPart(key string, val int, signals ...string) {
	if acc == nil || val <= 0 {
		return
	}
	if prev, ok := acc.scoreParts[key]; !ok || val > prev {
		acc.scoreParts[key] = val
	}
	for _, sig := range signals {
		if sig == "" {
			continue
		}
		if !containsStr(acc.signals, sig) {
			acc.signals = append(acc.signals, sig)
		}
	}
}

// SuggestForOrderItem returns ranked candidates for one order line.
func (s *Service) SuggestForOrderItem(ctx context.Context, orderItemID uuid.UUID, opts SuggestOpts) (*ItemCandidatesDTO, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("skucandidate: unavailable")
	}
	ro := opts.normalized()

	var it order.OrderItem
	if err := s.DB.WithContext(ctx).First(&it, "id = ?", orderItemID).Error; err != nil {
		return nil, err
	}
	var o order.Order
	if err := s.DB.WithContext(ctx).First(&o, "id = ? AND deleted_at IS NULL", it.OrderID).Error; err != nil {
		return nil, err
	}

	pool := map[uuid.UUID]*candidateAcc{}
	titleCtx := buildOrderLineText(&it)
	orderAttr := mergeAttrSignalsForOrderLine(it.SKUName, it.Attrs, rawAttrsSnippet(it.RawData))

	plat := strings.TrimSpace(o.Platform)
	shopNil := o.ShopID == nil || *o.ShopID == uuid.Nil

	lineExt := ""
	if it.ExternalSKUID != nil {
		lineExt = strings.TrimSpace(*it.ExternalSKUID)
	}
	seller := strings.TrimSpace(it.SellerSKU)
	code := strings.TrimSpace(it.SKUCode)
	codeOrSeller := code
	if codeOrSeller == "" {
		codeOrSeller = seller
	}

	if plat != "" && !shopNil {
		if err := s.addHistoryManualCandidates(ctx, pool, plat, *o.ShopID, lineExt, seller, code); err != nil {
			return nil, err
		}
	}

	if plat != "" && plat != "manual" && !shopNil {
		if err := s.addPublicationCandidates(ctx, pool, plat, *o.ShopID, lineExt, codeOrSeller); err != nil {
			return nil, err
		}
	}

	if codeOrSeller != "" {
		if err := s.addLocalSkuCodeCandidates(ctx, pool, codeOrSeller); err != nil {
			return nil, err
		}
	}

	if err := s.addTitleAndAttrCandidates(ctx, pool, titleCtx, orderAttr); err != nil {
		return nil, err
	}

	var list []CandidateDTO
	for _, acc := range pool {
		conf, _ := mergeConfidence(acc.scoreParts, acc.signals)
		if conf <= 0 {
			continue
		}
		if !ro.IncludeLowConfidence && conf < defaultMinConfidence {
			continue
		}
		dto, err := s.buildCandidateDTO(ctx, acc, conf)
		if err != nil || dto == nil {
			continue
		}
		list = append(list, *dto)
	}
	list = sortAndTrimCandidates(list, ro.Limit)

	return &ItemCandidatesDTO{OrderItemID: orderItemID.String(), List: list}, nil
}

func (s *Service) addHistoryManualCandidates(ctx context.Context, pool map[uuid.UUID]*candidateAcc, platform string, shopID uuid.UUID, ext, seller, skuCode string) error {
	var rows []struct {
		PSKU *uuid.UUID `gorm:"column:product_sku_id"`
		EID  *string    `gorm:"column:external_sku_id"`
	}
	tx := s.DB.WithContext(ctx).Table("order_item_sku_matches AS m").
		Select("m.product_sku_id, m.external_sku_id").
		Joins("JOIN orders mo ON mo.id = m.order_id AND mo.deleted_at IS NULL").
		Where(`m.match_type = ? AND m.match_status = ? AND m.platform = ? AND mo.shop_id = ? AND m.product_sku_id IS NOT NULL`,
			order.MatchTypeManual, order.MatchStatusManualBound, platform, shopID)

	var ors []string
	var args []any
	if ext != "" {
		ors = append(ors, "(m.external_sku_id IS NOT NULL AND LOWER(TRIM(m.external_sku_id)) = LOWER(?))")
		args = append(args, ext)
	}
	if seller != "" {
		ors = append(ors, "(m.seller_sku IS NOT NULL AND LOWER(TRIM(m.seller_sku)) = LOWER(?))")
		args = append(args, seller)
	}
	if skuCode != "" {
		ors = append(ors, "(m.sku_code IS NOT NULL AND LOWER(TRIM(m.sku_code)) = LOWER(?))")
		args = append(args, skuCode)
	}
	if len(ors) == 0 {
		return nil
	}
	tx = tx.Where("("+strings.Join(ors, " OR ")+")", args...)
	if err := tx.Scan(&rows).Error; err != nil {
		return err
	}

	for _, r := range rows {
		if r.PSKU == nil || *r.PSKU == uuid.Nil {
			continue
		}
		var pid uuid.UUID
		if err := s.DB.WithContext(ctx).Model(&product.ProductSKU{}).
			Select("product_id").Where("id = ? AND deleted_at IS NULL", *r.PSKU).Scan(&pid).Error; err != nil {
			continue
		}
		acc := s.accumulate(pool, *r.PSKU, pid)

		histScore := 96
		sig := "historical_manual_bound"
		if ext != "" && r.EID != nil && strings.EqualFold(strings.TrimSpace(*r.EID), ext) {
			histScore = 100
			sig = "historical_manual_bound+external_sku_id"
		}
		acc.addPart("history_manual_bind", histScore, sig)
	}
	return nil
}

func (s *Service) addPublicationCandidates(ctx context.Context, pool map[uuid.UUID]*candidateAcc, platform string, shopID uuid.UUID, ext, codeOrSeller string) error {
	if ext != "" {
		var rows []struct {
			PSKU *uuid.UUID `gorm:"column:product_sku_id"`
			PID  uuid.UUID  `gorm:"column:product_id"`
		}
		err := s.DB.WithContext(ctx).Table("product_publication_skus AS pps").
			Select("pps.product_sku_id, skus.product_id AS product_id").
			Joins("JOIN product_publications pp ON pp.id = pps.publication_id AND pp.deleted_at IS NULL").
			Joins("JOIN product_skus skus ON skus.id = pps.product_sku_id AND skus.deleted_at IS NULL").
			Where("pp.platform = ? AND pp.shop_id = ? AND pps.external_sku_id = ? AND pps.product_sku_id IS NOT NULL", platform, shopID, ext).
			Scan(&rows).Error
		if err != nil {
			return err
		}
		for _, r := range rows {
			if r.PSKU == nil {
				continue
			}
			acc := s.accumulate(pool, *r.PSKU, r.PID)
			acc.addPart("publication_external", 100, "external_sku_id_equal")
		}
	}

	if codeOrSeller != "" {
		norm := normalizeSKUCode(codeOrSeller)

		var rows []struct {
			PSKU *uuid.UUID `gorm:"column:product_sku_id"`
			PID  uuid.UUID  `gorm:"column:product_id"`
		}
		err := s.DB.WithContext(ctx).Table("product_publication_skus AS pps").
			Select("pps.product_sku_id, skus.product_id AS product_id").
			Joins("JOIN product_publications pp ON pp.id = pps.publication_id AND pp.deleted_at IS NULL").
			Joins("JOIN product_skus skus ON skus.id = pps.product_sku_id AND skus.deleted_at IS NULL").
			Where("pp.platform = ? AND pp.shop_id = ? AND LOWER(TRIM(pps.sku_code)) = LOWER(?) AND pps.product_sku_id IS NOT NULL", platform, shopID, codeOrSeller).
			Scan(&rows).Error
		if err != nil {
			return err
		}
		for _, r := range rows {
			if r.PSKU == nil {
				continue
			}
			acc := s.accumulate(pool, *r.PSKU, r.PID)
			acc.addPart("publication_sku_exact", 90, "sku_code_equal")
		}

		if norm != "" {
			var pubRows []struct {
				PubCode string     `gorm:"column:pub_code"`
				PSKU    *uuid.UUID `gorm:"column:product_sku_id"`
				PID     uuid.UUID  `gorm:"column:product_id"`
			}
			if err := s.DB.WithContext(ctx).Table("product_publication_skus AS pps").
				Select("pps.sku_code AS pub_code, pps.product_sku_id AS product_sku_id, skus.product_id AS product_id").
				Joins("JOIN product_publications pp ON pp.id = pps.publication_id AND pp.deleted_at IS NULL").
				Joins("JOIN product_skus skus ON skus.id = pps.product_sku_id AND skus.deleted_at IS NULL").
				Where("pp.platform = ? AND pp.shop_id = ? AND pps.product_sku_id IS NOT NULL AND pps.sku_code IS NOT NULL AND pps.sku_code <> ?", platform, shopID, "").
				Scan(&pubRows).Error; err != nil {
				return err
			}
			codeTrim := strings.TrimSpace(codeOrSeller)
			for _, r := range pubRows {
				if r.PSKU == nil {
					continue
				}
				if normalizeSKUCode(r.PubCode) != norm {
					continue
				}
				if strings.EqualFold(strings.TrimSpace(r.PubCode), codeTrim) {
					continue
				}
				acc := s.accumulate(pool, *r.PSKU, r.PID)
				acc.addPart("publication_sku_norm", 85, "normalized_sku_code_equal")
			}
		}
	}
	return nil
}

func (s *Service) addLocalSkuCodeCandidates(ctx context.Context, pool map[uuid.UUID]*candidateAcc, codeOrSeller string) error {
	norm := normalizeSKUCode(codeOrSeller)
	var skus []product.ProductSKU
	if err := s.DB.WithContext(ctx).Where("deleted_at IS NULL AND LOWER(TRIM(sku_code)) = LOWER(?)", codeOrSeller).
		Order("created_at ASC, id ASC").Find(&skus).Error; err != nil {
		return err
	}
	for _, sku := range skus {
		acc := s.accumulate(pool, sku.ID, sku.ProductID)
		acc.addPart("local_sku_exact", 80, "sku_code_equal")
	}
	if norm == "" {
		return nil
	}
	var all []product.ProductSKU
	if err := s.DB.WithContext(ctx).Where("deleted_at IS NULL AND sku_code IS NOT NULL AND sku_code <> ''").Find(&all).Error; err != nil {
		return err
	}
	for _, sku := range all {
		if normalizeSKUCode(sku.SKUCode) != norm {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(sku.SKUCode), strings.TrimSpace(codeOrSeller)) {
			continue
		}
		acc := s.accumulate(pool, sku.ID, sku.ProductID)
		acc.addPart("local_sku_norm", 75, "normalized_sku_code_equal")
	}
	return nil
}

func (s *Service) addTitleAndAttrCandidates(ctx context.Context, pool map[uuid.UUID]*candidateAcc, titleBlob string, orderAttr attrSignals) error {
	toks := tokenize(titleBlob)
	var qtoks []string
	seenTok := map[string]struct{}{}
	for _, t := range toks {
		if len([]rune(t)) < 2 {
			continue
		}
		if _, ok := seenTok[t]; ok {
			continue
		}
		seenTok[t] = struct{}{}
		qtoks = append(qtoks, t)
		if len(qtoks) >= 8 {
			break
		}
	}

	var productIDs []uuid.UUID
	pidSeen := map[uuid.UUID]struct{}{}

	for _, tk := range qtoks {
		if len(tk) > 48 {
			tk = string([]rune(tk)[:48])
		}
		likeTok := strings.ReplaceAll(strings.ReplaceAll(tk, `\`, `\\`), `%`, `\%`)
		likeTok = strings.ReplaceAll(likeTok, `_`, `\_`)

		tx := s.DB.WithContext(ctx).Model(&product.Product{}).Select("id").Where("deleted_at IS NULL").
			Where("(LOWER(title) LIKE ? OR LOWER(original_title) LIKE ? OR LOWER(ai_title) LIKE ?)",
				"%"+likeTok+"%", "%"+likeTok+"%", "%"+likeTok+"%").
			Limit(40)

		var ids []uuid.UUID
		if err := tx.Scan(&ids).Error; err != nil {
			return err
		}
		for _, id := range ids {
			if _, ok := pidSeen[id]; ok {
				continue
			}
			pidSeen[id] = struct{}{}
			productIDs = append(productIDs, id)
			if len(productIDs) >= 150 {
				break
			}
		}
		if len(productIDs) >= 150 {
			break
		}
	}

	if len(productIDs) == 0 {
		for skuID := range pool {
			p := pool[skuID]
			if _, ok := pidSeen[p.productID]; ok {
				continue
			}
			pidSeen[p.productID] = struct{}{}
			productIDs = append(productIDs, p.productID)
		}
	}
	if len(productIDs) == 0 {
		return nil
	}

	var products []product.Product
	if err := s.DB.WithContext(ctx).Preload("SKUs", "deleted_at IS NULL").
		Where("id IN ? AND deleted_at IS NULL", productIDs).Find(&products).Error; err != nil {
		return err
	}

	orderText := titleBlob
	for _, p := range products {
		j := bestProductTitleOverlap(orderText, p.Title, p.OriginalTitle, p.AITitle)
		titlePart := titleScoreFromJaccard(j)
		for _, sku := range p.SKUs {
			acc := s.accumulate(pool, sku.ID, sku.ProductID)
			if titlePart > 0 {
				acc.addPart("title", titlePart, "title_token_overlap")
			}
			skuAttr := mergeAttrSignalsForOrderLine(sku.SKUName, sku.Attrs, "")
			if sc, sigs := attrsSimilaritySignals(orderAttr, skuAttr); sc > 0 {
				acc.addPart("attrs", sc, sigs...)
			}
		}
	}
	return nil
}

func (s *Service) buildCandidateDTO(ctx context.Context, acc *candidateAcc, conf int) (*CandidateDTO, error) {
	var sku product.ProductSKU
	if err := s.DB.WithContext(ctx).First(&sku, "id = ? AND deleted_at IS NULL", acc.skuID).Error; err != nil {
		return nil, err
	}
	var p product.Product
	if err := s.DB.WithContext(ctx).First(&p, "id = ? AND deleted_at IS NULL", sku.ProductID).Error; err != nil {
		return nil, err
	}

	confF, _ := mergeConfidence(acc.scoreParts, acc.signals)
	if confF != conf {
		confF = conf
	}
	if confF <= 0 {
		return nil, nil
	}

	src := primarySource(acc.scoreParts)
	reason := buildReason(src, confF, acc.signals)

	var attrsOut map[string]any
	if len(sku.Attrs) > 0 {
		_ = json.Unmarshal(sku.Attrs, &attrsOut)
	}

	breakdown := map[string]int{}
	for k, v := range acc.scoreParts {
		if v > 0 {
			breakdown[k] = v
		}
	}

	return &CandidateDTO{
		ProductID:       p.ID.String(),
		ProductTitle:    strings.TrimSpace(p.Title),
		ProductSKUID:    sku.ID.String(),
		SKUCode:         strings.TrimSpace(sku.SKUCode),
		SKUName:         strings.TrimSpace(sku.SKUName),
		Stock:           sku.Stock,
		Attrs:           attrsOut,
		Confidence:      confF,
		Reason:          reason,
		MatchSignals:    append([]string{}, acc.signals...),
		Source:          src,
		SourceBreakdown: breakdown,
	}, nil
}

func buildOrderLineText(it *order.OrderItem) string {
	if it == nil {
		return ""
	}
	var parts []string
	for _, s := range []string{strings.TrimSpace(it.ProductTitle), strings.TrimSpace(it.SKUName)} {
		if s != "" {
			parts = append(parts, s)
		}
	}
	if h := titleFromRaw(it.RawData); h != "" {
		parts = append(parts, h)
	}
	return strings.TrimSpace(strings.Join(parts, " "))
}

func titleFromRaw(raw datatypes.JSON) string {
	if len(raw) > 65536 {
		return ""
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return ""
	}
	tryKeys := []string{"product_title", "item_title", "title", "sku_title", "item_name", "productTitle", "skuName"}
	var best string
	for _, k := range tryKeys {
		v, ok := m[k].(string)
		if !ok {
			continue
		}
		v = strings.TrimSpace(v)
		if len(v) > len(best) && len([]rune(v)) <= 512 {
			best = v
		}
	}
	return clipRunes(best, 512)
}

func rawAttrsSnippet(raw datatypes.JSON) string {
	if len(raw) > 65536 {
		return ""
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return ""
	}
	sub, ok := m["attrs"]
	if !ok {
		sub = m["attributes"]
	}
	if sub == nil {
		sub = m["variation"]
	}
	if sub == nil {
		return ""
	}
	bs, err := json.Marshal(sub)
	if err != nil || len(bs) > 1024 {
		return ""
	}
	return string(bs)
}

func clipRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}

// BatchForOrder verifies all items belong to the order before suggesting.
func (s *Service) BatchForOrder(ctx context.Context, orderID uuid.UUID, body BatchRequest) (*BatchResponse, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("skucandidate: unavailable")
	}
	limit := body.Limit
	if limit <= 0 {
		limit = 10
	}
	ilc := body.IncludeLowConfidence != nil && *body.IncludeLowConfidence

	var out []ItemCandidatesDTO
	for _, sid := range body.OrderItemIDs {
		sid = strings.TrimSpace(sid)
		if sid == "" {
			continue
		}
		iid, err := uuid.Parse(sid)
		if err != nil {
			return nil, fmt.Errorf("invalid orderItemId")
		}
		var oid uuid.UUID
		if err := s.DB.WithContext(ctx).Model(&order.OrderItem{}).
			Select("order_id").Where("id = ?", iid).Scan(&oid).Error; err != nil || oid != orderID {
			return nil, fmt.Errorf("orderItemId does not belong to order")
		}
		dto, err := s.SuggestForOrderItem(ctx, iid, SuggestOpts{Limit: limit, IncludeLowConfidence: ilc})
		if err != nil {
			return nil, err
		}
		out = append(out, *dto)
	}
	return &BatchResponse{OrderID: orderID.String(), Items: out}, nil
}
