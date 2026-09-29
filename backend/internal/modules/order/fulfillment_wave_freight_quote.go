package order

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/logistics"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type FulfillmentWaveFreightQuoteResult struct {
	DestinationCountryCode string                     `json:"destinationCountryCode"`
	DestinationRegion      string                     `json:"destinationRegion,omitempty"`
	DestinationPostalCode  string                     `json:"destinationPostalCode,omitempty"`
	Candidates             []logistics.QuoteCandidate `json:"candidates"`
}

type ConfirmFulfillmentWaveFreightQuoteInput struct {
	ExpectedRevision     int       `json:"expectedRevision"`
	IdempotencyKey       string    `json:"idempotencyKey"`
	RateTemplateID       uuid.UUID `json:"rateTemplateId"`
	RateTemplateRevision int       `json:"rateTemplateRevision"`
	WeightGrams          int       `json:"weightGrams"`
}

func (s *Service) freightQuoteFacts(ctx context.Context, tenantID int64, principal *adminperm.Principal, waveID, orderID uuid.UUID) (*FulfillmentWave, *FulfillmentWaveOrder, *Order, error) {
	if s == nil || s.DB == nil || s.Logistics == nil || waveID == uuid.Nil || orderID == uuid.Nil {
		return nil, nil, nil, ErrFulfillmentWaveInvalidInput
	}
	wave, err := s.GetFulfillmentWave(ctx, tenantID, principal, waveID)
	if err != nil {
		return nil, nil, nil, err
	}
	var waveOrder *FulfillmentWaveOrder
	for i := range wave.Orders {
		if wave.Orders[i].OrderID == orderID {
			waveOrder = &wave.Orders[i]
			break
		}
	}
	if waveOrder == nil {
		return nil, nil, nil, ErrFulfillmentWaveNotFound
	}
	var orderRow Order
	if err := s.DB.WithContext(ctx).Where("tenant_id = ? AND id = ?", tenantID, orderID).First(&orderRow).Error; err != nil {
		return nil, nil, nil, ErrFulfillmentWaveNotFound
	}
	return wave, waveOrder, &orderRow, nil
}

func (s *Service) QuoteFulfillmentWaveFreight(ctx context.Context, tenantID int64, principal *adminperm.Principal, waveID, orderID uuid.UUID, weightGrams int) (*FulfillmentWaveFreightQuoteResult, error) {
	wave, _, orderRow, err := s.freightQuoteFacts(ctx, tenantID, principal, waveID, orderID)
	if err != nil {
		return nil, err
	}
	quotes, err := s.Logistics.Quote(ctx, tenantID, logistics.QuoteInput{WarehouseID: wave.WarehouseID, CountryCode: orderRow.DestinationCountryCode, Region: orderRow.DestinationRegion, PostalCode: orderRow.DestinationPostalCode, WeightGrams: weightGrams})
	if err != nil {
		return nil, err
	}
	return &FulfillmentWaveFreightQuoteResult{DestinationCountryCode: orderRow.DestinationCountryCode, DestinationRegion: orderRow.DestinationRegion, DestinationPostalCode: orderRow.DestinationPostalCode, Candidates: quotes}, nil
}

func (s *Service) ConfirmFulfillmentWaveFreightQuote(ctx context.Context, tenantID int64, principal *adminperm.Principal, actor *uuid.UUID, waveID, orderID uuid.UUID, in ConfirmFulfillmentWaveFreightQuoteInput) (*FulfillmentWave, error) {
	wave, _, _, err := s.freightQuoteFacts(ctx, tenantID, principal, waveID, orderID)
	if err != nil {
		return nil, err
	}
	if err := requireFulfillmentWaveOperate(principal, wave); err != nil {
		return nil, err
	}
	key, keyErr := normalizeWaveKey(in.IdempotencyKey)
	if keyErr != nil || in.ExpectedRevision < 1 || in.RateTemplateID == uuid.Nil || in.RateTemplateRevision < 1 || in.WeightGrams <= 0 || in.WeightGrams > maxFulfillmentWaveWeightGrams {
		return nil, ErrFulfillmentWaveInvalidInput
	}
	hash, err := fulfillmentWavePayloadHash(struct {
		Action               string    `json:"action"`
		Revision             int       `json:"revision"`
		OrderID              uuid.UUID `json:"orderId"`
		RateTemplateID       uuid.UUID `json:"rateTemplateId"`
		RateTemplateRevision int       `json:"rateTemplateRevision"`
		WeightGrams          int       `json:"weightGrams"`
	}{"confirm_freight_quote", in.ExpectedRevision, orderID, in.RateTemplateID, in.RateTemplateRevision, in.WeightGrams})
	if err != nil {
		return nil, err
	}
	err = s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		lockedWave, err := getFulfillmentWaveTx(ctx, tx, tenantID, waveID)
		if err != nil {
			return err
		}
		if replay, err := findFulfillmentWaveActionTx(tx, tenantID, waveID, key, hash); err != nil || replay {
			return err
		}
		if lockedWave.Revision != in.ExpectedRevision {
			return ErrFulfillmentWaveRevision
		}
		if lockedWave.Status != FulfillmentWavePacking && lockedWave.Status != FulfillmentWavePartial {
			return ErrFulfillmentWaveState
		}
		var lockedOrder FulfillmentWaveOrder
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("tenant_id = ? AND wave_id = ? AND order_id = ?", tenantID, waveID, orderID).First(&lockedOrder).Error; err != nil {
			return ErrFulfillmentWaveNotFound
		}
		if lockedOrder.Status != FulfillmentWaveOrderReadyToPack && lockedOrder.Status != FulfillmentWaveOrderFailed {
			return ErrFulfillmentWavePackingIncomplete
		}
		var lockedOrderRow Order
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("tenant_id = ? AND id = ?", tenantID, orderID).First(&lockedOrderRow).Error; err != nil {
			return ErrFulfillmentWaveNotFound
		}
		quote, err := (&logistics.Service{DB: tx}).QuoteByRate(ctx, tenantID, in.RateTemplateID, logistics.QuoteInput{WarehouseID: lockedWave.WarehouseID, CountryCode: lockedOrderRow.DestinationCountryCode, Region: lockedOrderRow.DestinationRegion, PostalCode: lockedOrderRow.DestinationPostalCode, WeightGrams: in.WeightGrams})
		if err != nil {
			return err
		}
		if quote.RateTemplateRevision != in.RateTemplateRevision {
			return ErrFulfillmentWaveRevision
		}
		var version int
		if err := tx.Model(&FulfillmentWaveFreightQuote{}).Where("tenant_id = ? AND wave_id = ? AND wave_order_id = ?", tenantID, waveID, lockedOrder.ID).Select("COALESCE(MAX(version), 0)").Scan(&version).Error; err != nil {
			return err
		}
		resultRevision := lockedWave.Revision + 1
		result := tx.Model(&FulfillmentWave{}).Where("tenant_id = ? AND id = ? AND revision = ?", tenantID, waveID, lockedWave.Revision).Updates(map[string]any{"revision": resultRevision})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return ErrFulfillmentWaveRevision
		}
		action, err := createFulfillmentWaveActionTx(tx, tenantID, waveID, "confirm_freight_quote", key, hash, in.ExpectedRevision, resultRevision, actor)
		if err != nil {
			return err
		}
		row := FulfillmentWaveFreightQuote{TenantID: tenantID, WaveID: waveID, WaveOrderID: lockedOrder.ID, OrderID: orderID, Version: version + 1, SourceRevision: in.ExpectedRevision, ActionID: action.ID, WarehouseID: lockedWave.WarehouseID, RateTemplateID: quote.RateTemplateID, RateTemplateCode: quote.RateTemplateCode, RateTemplateName: quote.RateTemplateName, RateTemplateRevision: quote.RateTemplateRevision, ChannelID: quote.ChannelID, ChannelCode: quote.ChannelCode, ChannelName: quote.ChannelName, Carrier: quote.Carrier, DestinationCountryCode: strings.ToUpper(strings.TrimSpace(lockedOrderRow.DestinationCountryCode)), DestinationRegion: strings.TrimSpace(lockedOrderRow.DestinationRegion), DestinationPostalCode: strings.ToUpper(strings.TrimSpace(lockedOrderRow.DestinationPostalCode)), WeightGrams: quote.WeightGrams, MinWeightGrams: quote.MinWeightGrams, MaxWeightGrams: quote.MaxWeightGrams, AmountMinor: quote.AmountMinor, Currency: quote.Currency, Explanation: quote.Explanation, ActorID: actor}
		return tx.Create(&row).Error
	})
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrFulfillmentWaveNotFound
		}
		return nil, err
	}
	s.writeFulfillmentWaveLog(ctx, tenantID, actor, waveID, "confirm_freight_quote", fmt.Sprintf("orderId=%s rateTemplateId=%s weightGrams=%d", orderID, in.RateTemplateID, in.WeightGrams))
	return s.GetFulfillmentWave(ctx, tenantID, principal, waveID)
}
