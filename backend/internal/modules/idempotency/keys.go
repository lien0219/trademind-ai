package idempotency

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
)

// Key builders use stable business semantics; never embed secrets or PII.

// OrderSyncJob deduplicates shop-level sync task creation.
func OrderSyncJob(platform, shopID, syncMode, windowOrCursor string) string {
	return fmt.Sprintf("order-sync-job:%s:%s:%s:%s",
		norm(platform), norm(shopID), norm(syncMode), norm(windowOrCursor))
}

// OrderImport deduplicates single-order import/upsert.
func OrderImport(platform, shopID, platformOrderID string) string {
	return fmt.Sprintf("order-import:%s:%s:%s",
		norm(platform), norm(shopID), norm(platformOrderID))
}

// OrderImportRevision deduplicates a specific platform revision/update snapshot.
func OrderImportRevision(platform, shopID, platformOrderID, revisionOrUpdatedAt string) string {
	base := OrderImport(platform, shopID, platformOrderID)
	rev := norm(revisionOrUpdatedAt)
	if rev == "" {
		return base
	}
	return fmt.Sprintf("%s:%s", base, rev)
}

// WebhookDouyin scopes webhook event dedup per shop when shop id is known.
func WebhookDouyin(shopID, eventID string) string {
	return fmt.Sprintf("webhook:douyin:%s:%s", norm(shopID), norm(eventID))
}

// OrderSync is kept for per-order sync side effects inside a running job.
func OrderSync(platform, shopID, platformOrderID string) string {
	return OrderImport(platform, shopID, platformOrderID)
}

func InventoryDeduct(orderID, orderItemID, skuID string) string {
	return fmt.Sprintf("inventory-deduct:%s:%s:%s",
		norm(orderID), norm(orderItemID), norm(skuID))
}

// OrderFulfillment scopes a caller-generated fulfillment action to one order.
// The caller key is kept in the key so reusing it for another payload is
// rejected by the request-hash check rather than silently creating a shipment.
func OrderFulfillment(orderID, callerKey string) string {
	return fmt.Sprintf("order-fulfillment:%s:%s", norm(orderID), norm(callerKey))
}

// OrderWarehouseAllocation scopes a caller-generated manual allocation to one
// order. Request-hash validation rejects reuse for another warehouse/revision.
func OrderWarehouseAllocation(orderID, callerKey string) string {
	return fmt.Sprintf("order-warehouse-allocation:%s:%s", norm(orderID), norm(callerKey))
}

func InventoryCompensate(orderID, orderItemID, skuID, reason string) string {
	return fmt.Sprintf("inventory-compensate:%s:%s:%s:%s",
		norm(orderID), norm(orderItemID), norm(skuID), norm(reason))
}

func InventoryPush(platform, shopID, skuID, stockVersion string) string {
	return fmt.Sprintf("inventory-push:%s:%s:%s:%s",
		norm(platform), norm(shopID), norm(skuID), norm(stockVersion))
}

func PublishBatch(shopID, productDraftID, publishVersion string) string {
	return fmt.Sprintf("publish-batch:%s:%s:%s",
		norm(shopID), norm(productDraftID), norm(publishVersion))
}

func PublishEnqueue(publishBatchID, taskType string) string {
	return fmt.Sprintf("publish-enqueue:%s:%s",
		norm(publishBatchID), norm(taskType))
}

func CustomerSend(conversationID, clientMessageID string) string {
	return fmt.Sprintf("customer-send:%s:%s",
		norm(conversationID), norm(clientMessageID))
}

func AITextApply(batchID, itemID, targetProductID, targetVersion string) string {
	return fmt.Sprintf("ai-text-apply:%s:%s:%s:%s",
		norm(batchID), norm(itemID), norm(targetProductID), norm(targetVersion))
}

// LegacyAITextApply preserves the older 3-part key shape for migration / tests.
func LegacyAITextApply(batchID, itemID, targetVersion string) string {
	return fmt.Sprintf("ai-text-apply:%s:%s:%s",
		norm(batchID), norm(itemID), norm(targetVersion))
}

func AITextUndo(applyRecordID, targetVersion string) string {
	return fmt.Sprintf("ai-text-undo:%s:%s",
		norm(applyRecordID), norm(targetVersion))
}

func AIImageApply(batchID, itemID, targetProductID, targetVersion, slot string) string {
	return fmt.Sprintf("ai-image-apply:%s:%s:%s:%s:%s",
		norm(batchID), norm(itemID), norm(targetProductID), norm(targetVersion), norm(slot))
}

// LegacyAIImageApply preserves the older 3-part key shape for migration / tests.
func LegacyAIImageApply(batchID, itemID, targetVersion string) string {
	return fmt.Sprintf("ai-image-apply:%s:%s:%s",
		norm(batchID), norm(itemID), norm(targetVersion))
}

func AIImageUndo(applyRecordID, targetVersion string) string {
	return fmt.Sprintf("ai-image-undo:%s:%s",
		norm(applyRecordID), norm(targetVersion))
}

func AITextBatch(productID, productVersion, operationType, inputHash string) string {
	return fmt.Sprintf("ai-text-batch:%s:%s:%s:%s",
		norm(productID), norm(productVersion), norm(operationType), norm(inputHash))
}

func AIImageBatch(productID, productVersion, operationType, inputImageHash string) string {
	return fmt.Sprintf("ai-image-batch:%s:%s:%s:%s",
		norm(productID), norm(productVersion), norm(operationType), norm(inputImageHash))
}

// LegacyAITextBatch preserves older two-part key shape for migration compatibility.
func LegacyAITextBatch(productID, contentHash, operationType string) string {
	return fmt.Sprintf("ai-text-batch:%s:%s:%s",
		norm(productID), norm(contentHash), norm(operationType))
}

func Webhook(platform, eventID string) string {
	return fmt.Sprintf("webhook:%s:%s", norm(platform), norm(eventID))
}

// WebhookScoped deduplicates inbound webhook events per tenant/shop.
func WebhookScoped(platform string, tenantID int64, platformShopID, eventID string) string {
	return fmt.Sprintf("webhook:%s:%d:%s:%s", norm(platform), tenantID, norm(platformShopID), norm(eventID))
}

// WebhookProcess deduplicates async webhook event processing.
func WebhookProcess(platform, eventID string) string {
	return fmt.Sprintf("webhook-process:%s:%s", norm(platform), norm(eventID))
}

// WebhookProcessScoped deduplicates async webhook processing per tenant/shop.
func WebhookProcessScoped(platform string, tenantID int64, platformShopID, eventID string) string {
	return fmt.Sprintf("webhook-process:%s:%d:%s:%s", norm(platform), tenantID, norm(platformShopID), norm(eventID))
}

// DouyinProductDraftCreate deduplicates Douyin platform draft creation.
func DouyinProductDraftCreate(shopID, productDraftID, publishVersion string) string {
	return fmt.Sprintf("douyin-product-draft-create:%s:%s:%s",
		norm(shopID), norm(productDraftID), norm(publishVersion))
}

// DouyinImageUpload deduplicates Douyin image upload by content hash.
func DouyinImageUpload(shopID, storageObjectKey, contentHash string) string {
	return fmt.Sprintf("douyin-image-upload:%s:%s:%s",
		norm(shopID), norm(storageObjectKey), norm(contentHash))
}

// AIProductApply deduplicates product AI content application.
func AIProductApply(productID, fieldType, taskID, sourceSnapshotHash string) string {
	return fmt.Sprintf("ai-product-apply:%s:%s:%s:%s",
		norm(productID), norm(fieldType), norm(taskID), norm(sourceSnapshotHash))
}

// HashRequest returns a stable SHA-256 hex digest of normalized request payload bytes.
func HashRequest(payload []byte) string {
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}

func norm(s string) string {
	return strings.TrimSpace(s)
}
