/// <reference types="@umijs/max/typings" />

declare namespace API {
  type CurrentUser = {
    id: string;
    username: string; // login identifier (email or phone)
    email?: string;
    phone?: string;
    displayName: string;
    createdAt?: string;
    updatedAt?: string;
  };
}
