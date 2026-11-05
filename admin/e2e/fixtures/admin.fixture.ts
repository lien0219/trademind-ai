import { test as base } from "@playwright/test";
import { AdminPage } from "../pages/admin.page";

type AdminFixtures = {
  admin: AdminPage;
  autoAdminGuards: void;
  anonymousRoutePaths: string[];
};

export const test = base.extend<AdminFixtures>({
  anonymousRoutePaths: [[], { option: true }],
  admin: async ({ page }, use) => {
    await use(new AdminPage(page));
  },
  autoAdminGuards: [
    async ({ admin, anonymousRoutePaths }, use) => {
      await admin.setup(anonymousRoutePaths);
      await use();
      await admin.writeGuard.expectNoUnexpectedWrites();
      await admin.consoleGuard.expectNoFatalErrors();
    },
    { auto: true },
  ],
});

export { expect } from "@playwright/test";
