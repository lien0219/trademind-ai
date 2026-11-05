/**
 * A finance import preview is an authorization to confirm only the exact
 * uploaded file, target shop and server-validated hashes in one tenant.
 * Keep the scope by reference so a delayed confirmation dialog cannot reuse a
 * newer preview after the user changes files, shops or tenant context.
 */
export type FinanceImportPreviewScope = {
  file: File;
  shopId: string;
  fileHash: string;
  calculationHash?: string;
  idempotencyKey: string;
  isCurrent: () => boolean;
};

export function canConfirmFinanceImport(
  current: FinanceImportPreviewScope | undefined,
  expected: FinanceImportPreviewScope | undefined,
  input: {
    file: File;
    shopId: string;
    fileHash: string;
    calculationHash?: string;
    idempotencyKey: string;
  },
): boolean {
  return Boolean(
    current &&
    expected &&
    current === expected &&
    current.isCurrent() &&
    current.file === input.file &&
    current.shopId === input.shopId &&
    current.fileHash === input.fileHash &&
    current.calculationHash === input.calculationHash &&
    current.idempotencyKey === input.idempotencyKey,
  );
}
