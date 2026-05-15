/// <reference types="@umijs/max/typings" />

declare namespace API {
  type CurrentUser = {
    id: string;
    username: string;
    displayName: string;
    createdAt?: string;
    updatedAt?: string;
  };
}
