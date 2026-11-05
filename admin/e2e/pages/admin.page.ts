import { expect, type Page } from "@playwright/test";
import { seedAdminAuth, routeAdminApi } from "../utils/routes";
import { ConsoleGuard } from "../utils/console-guard";
import { NetworkWriteGuard } from "../utils/network-guard";

export class AdminPage {
  readonly consoleGuard: ConsoleGuard;
  readonly writeGuard: NetworkWriteGuard;

  constructor(readonly page: Page) {
    this.consoleGuard = new ConsoleGuard(page);
    this.writeGuard = new NetworkWriteGuard(page);
  }

  async setup(anonymousRoutePaths: string[] = []) {
    this.consoleGuard.install();
    await seedAdminAuth(this.page, anonymousRoutePaths);
    await this.writeGuard.install();
    await routeAdminApi(this.page);
  }

  async goto(path: string) {
    await this.page.goto(path);
    await expect(this.page).not.toHaveURL(/\/user\/login/);
  }
}
