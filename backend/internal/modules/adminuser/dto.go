package adminuser

// UserRow is a list/detail DTO.
type UserRow struct {
	ID               string      `json:"id"`
	Username         string      `json:"username"`
	Email            string      `json:"email,omitempty"`
	Phone            string      `json:"phone,omitempty"`
	DisplayName      string      `json:"displayName"`
	Role             string      `json:"role"`
	Status           string      `json:"status"`
	StorePermissions []StorePerm `json:"storePermissions,omitempty"`
	LastLoginAt      *string     `json:"lastLoginAt,omitempty"`
	LastOperationAt  *string     `json:"lastOperationAt,omitempty"`
	CreatedAt        string      `json:"createdAt"`
	UpdatedAt        string      `json:"updatedAt"`
}

// StorePerm is a store authorization row.
type StorePerm struct {
	ID              string `json:"id"`
	StoreID         string `json:"storeId"`
	StoreName       string `json:"storeName,omitempty"`
	Platform        string `json:"platform,omitempty"`
	PermissionScope string `json:"permissionScope"`
}
