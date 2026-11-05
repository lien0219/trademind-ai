import { beforeEach, describe, expect, it, vi } from "vitest";

const launchPersistentContext = vi.fn();
const installPublicNetworkGuard = vi.fn();

vi.mock("playwright", () => ({
  chromium: { launchPersistentContext },
}));

vi.mock("../../security/public-url.js", () => ({
  installPublicNetworkGuard,
}));

vi.mock("../../config/env.js", () => ({
  getBrowserHeadless: () => true,
  getDefaultNavigationTimeoutMs: () => 12_345,
}));

vi.mock("../browser-paths.js", () => ({
  ensureBrowserDataDirs: vi.fn(),
  get1688UserDataDir: () => "e2e-profiles/1688",
  getPinduoduoUserDataDir: () => "e2e-profiles/pinduoduo",
  getTaobaoTmallUserDataDir: () => "e2e-profiles/taobao_tmall",
  getCustomProfileUserDataDir: () => "e2e-profiles/custom/e2e-profile",
}));

function createContext() {
  let closed = false;
  return {
    addInitScript: vi.fn().mockResolvedValue(undefined),
    setDefaultNavigationTimeout: vi.fn(),
    setDefaultTimeout: vi.fn(),
    isClosed: () => closed,
    close: vi.fn().mockImplementation(async () => { closed = true; }),
  };
}

describe.each(["provider", "custom profile"] as const)("%s persistent context", (kind) => {
  beforeEach(() => {
    vi.resetAllMocks();
    installPublicNetworkGuard.mockResolvedValue(undefined);
  });

  async function createManager() {
    if (kind === "provider") {
      const { BrowserSessionManager } = await import("../session-manager.js");
      const manager = new BrowserSessionManager();
      return {
        get: (options?: { headless?: boolean }) => manager.getOrCreateProviderContext("pinduoduo", options),
        close: () => manager.close(),
      };
    }
    const { CustomProfileSessionManager } = await import("../profile-sessions.js");
    const manager = new CustomProfileSessionManager();
    return {
      get: (options?: { headless?: boolean }) => manager.getOrCreateContext("e2e-profile", options),
      close: () => manager.close(),
    };
  }

  it.each(["initialization", "network guard", "timeout configuration"])(
    "releases a context after %s fails so the profile can be retried",
    async (stage) => {
      const failedContext = createContext();
      const recoveredContext = createContext();
      const failure = new Error(`${stage} failed`);
      if (stage === "initialization") failedContext.addInitScript.mockRejectedValueOnce(failure);
      else if (stage === "network guard") installPublicNetworkGuard.mockRejectedValueOnce(failure);
      else failedContext.setDefaultTimeout.mockImplementationOnce(() => { throw failure; });
      launchPersistentContext.mockResolvedValueOnce(failedContext).mockResolvedValueOnce(recoveredContext);
      const manager = await createManager();

      await expect(manager.get()).rejects.toBe(failure);
      expect(failedContext.isClosed()).toBe(true);
      expect(await manager.get()).toBe(recoveredContext);
      expect(launchPersistentContext).toHaveBeenCalledTimes(2);
      expect(recoveredContext.isClosed()).toBe(false);
      await manager.close();
      expect(recoveredContext.isClosed()).toBe(true);
    },
  );

  it("preserves the initialization error when context cleanup also fails", async () => {
    const context = createContext();
    const failure = new Error("initialization failed");
    context.addInitScript.mockRejectedValueOnce(failure);
    context.close.mockRejectedValueOnce(new Error("cleanup failed"));
    launchPersistentContext.mockResolvedValueOnce(context);
    const manager = await createManager();

    await expect(manager.get()).rejects.toBe(failure);
    expect(context.close).toHaveBeenCalledTimes(1);
    await manager.close();
    expect(context.close).toHaveBeenCalledTimes(1);
  });

  it("reuses a guarded profile with its viewport and headless preference intact", async () => {
    const context = createContext();
    launchPersistentContext.mockResolvedValue(context);
    const manager = await createManager();

    expect(await manager.get({ headless: false })).toBe(context);
    expect(await manager.get({ headless: false })).toBe(context);
    expect(launchPersistentContext).toHaveBeenCalledTimes(1);
    expect(launchPersistentContext).toHaveBeenCalledWith(
      kind === "provider" ? "e2e-profiles/pinduoduo" : "e2e-profiles/custom/e2e-profile",
      expect.objectContaining({
        headless: false,
        locale: "zh-CN",
        serviceWorkers: "block",
        viewport: { width: 1280, height: kind === "provider" ? 900 : 800 },
      }),
    );
    expect(installPublicNetworkGuard).toHaveBeenCalledWith(context);
    expect(context.isClosed()).toBe(false);
    await manager.close();
    expect(context.isClosed()).toBe(true);
  });
});
