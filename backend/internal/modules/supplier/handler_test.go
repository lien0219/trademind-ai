package supplier

import "testing"

func TestMaskSupplierContact(t *testing.T) {
	row := &Supplier{Phone: "13812345678", Email: "buyer@example.com"}
	maskSupplierContact(row)
	if row.Phone != "138****5678" {
		t.Fatalf("unexpected masked phone: %q", row.Phone)
	}
	if row.Email != "bu***@example.com" {
		t.Fatalf("unexpected masked email: %q", row.Email)
	}
}
