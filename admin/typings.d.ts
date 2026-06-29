/// <reference types="@umijs/max/typings" />

declare namespace API {
  type StorePermission = {
    storeId: string;
    platform?: string;
    permissionScope: string;
  };

  type CurrentUser = {
    id: string;
    username: string; // login identifier (email or phone)
    email?: string;
    phone?: string;
    displayName: string;
    role?: string;
    status?: string;
    permissions?: string[];
    storePermissions?: StorePermission[];
    createdAt?: string;
    updatedAt?: string;
  };
}
