// Keep the shared confirmation guard independent from domain service imports.
// The warehouse fee API preview is structurally compatible with these fields.
type ConfirmationFields = {
  orderId: string;
  rateCardId: string;
  rateCardRevision: number;
  waveRevision: number;
  calculationHash: string;
};

export type WarehouseFeeConfirmationScope = {
  preview: ConfirmationFields;
  signature: string;
  isCurrent: () => boolean;
};

function confirmationSignature(preview: ConfirmationFields): string {
  return JSON.stringify([
    preview.orderId,
    preview.rateCardId,
    preview.rateCardRevision,
    preview.waveRevision,
    preview.calculationHash,
  ]);
}

/** A confirmed fee must use the exact server preview validated in this tenant. */
export function createWarehouseFeeConfirmationScope(
  preview: ConfirmationFields,
  isCurrent: () => boolean,
): WarehouseFeeConfirmationScope {
  return { preview, signature: confirmationSignature(preview), isCurrent };
}

export function canConfirmWarehouseFeePreview(
  scope: WarehouseFeeConfirmationScope | undefined,
  preview: ConfirmationFields | undefined,
): boolean {
  return Boolean(
    scope &&
    preview &&
    scope.preview === preview &&
    scope.isCurrent() &&
    scope.signature === confirmationSignature(preview),
  );
}
