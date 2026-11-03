import { beforeEach, describe, expect, it, vi } from "vitest";

const launch = vi.fn();
const installPublicNetworkGuard = vi.fn();

vi.mock("playwright", () => ({
  chromium: { launch },
}));

vi.mock("../../security/public-url.js", () => ({
  installPublicNetworkGuard,
}));

vi.mock("../../config/env.js", () => ({
  getBrowserHeadless: () => true,
  getDefaultNavigationTimeoutMs: () => 12_345,
}));

function createContext() {
  const page = {
    setDefaultNavigationTimeout: vi.fn(),
    setDefaultTimeout: vi.fn(),
    close: vi.fn().mockResolvedValue(undefined),
  };
  const context = {
    newPage: vi.fn().mockResolvedValue(page),
    addInitScript: vi.fn().mockResolvedValue(undefined),
    route: vi.fn().mockResolvedValue(undefined),
    routeWebSocket: vi.fn().mockResolvedValue(undefined),
    close: vi.fn().mockResolvedValue(undefined),
  };
  return { context, page };
}

describe("BrowserManager unprofiled pages", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("installs the guarded context and preserves context options and timeout", async () => {
    const { context, page } = createContext();
    const browser = {
      newContext: vi.fn().mockResolvedValue(context),
    };
    launch.mockResolvedValue(browser);

    const { BrowserManager } = await import("../manager.js");
    const manager = new BrowserManager();
    const result = await manager.withUnprofiledPage(
      async (receivedPage) => receivedPage,
      {
        userAgent: "test-agent",
        locale: "en-US",
        viewport: { width: 1024, height: 768 },
        isMobile: false,
        hasTouch: false,
        timeoutMs: 4_321,
      },
    );

    expect(result).toBe(page);
    expect(browser.newContext).toHaveBeenCalledWith({
      userAgent: "test-agent",
      locale: "en-US",
      viewport: { width: 1024, height: 768 },
      isMobile: false,
      hasTouch: false,
      serviceWorkers: "block",
    });
    expect(context.addInitScript).toHaveBeenCalledTimes(1);
    expect(installPublicNetworkGuard).toHaveBeenCalledWith(context);
    expect(page.setDefaultNavigationTimeout).toHaveBeenCalledWith(4_321);
    expect(page.setDefaultTimeout).toHaveBeenCalledWith(4_321);
    expect(page.close).toHaveBeenCalledTimes(1);
    expect(context.close).toHaveBeenCalledTimes(1);
  });

  it("closes the page and context when the callback fails", async () => {
    const { context, page } = createContext();
    const browser = {
      newContext: vi.fn().mockResolvedValue(context),
    };
    launch.mockResolvedValue(browser);

    const { BrowserManager } = await import("../manager.js");
    const manager = new BrowserManager();
    const failure = new Error("callback failed");

    await expect(
      manager.withUnprofiledPage(async () => {
        throw failure;
      }),
    ).rejects.toBe(failure);

    expect(page.close).toHaveBeenCalledTimes(1);
    expect(context.close).toHaveBeenCalledTimes(1);
  });

  it("blocks private HTTP and WebSocket destinations before using the page", async () => {
    const { context } = createContext();
    launch.mockResolvedValue({ newContext: vi.fn().mockResolvedValue(context) });
    const publicUrl = await vi.importActual<typeof import("../../security/public-url.js")>(
      "../../security/public-url.js",
    );
    installPublicNetworkGuard.mockImplementationOnce(publicUrl.installPublicNetworkGuard);
    const { BrowserManager } = await import("../manager.js");

    await new BrowserManager().withUnprofiledPage(async () => {
      const httpHandler = context.route.mock.calls[0][1];
      const httpRoute = {
        request: () => ({ url: () => "http://127.0.0.1/private" }),
        abort: vi.fn().mockResolvedValue(undefined),
        continue: vi.fn().mockResolvedValue(undefined),
      };
      await httpHandler(httpRoute);
      expect(httpRoute.abort).toHaveBeenCalledWith("blockedbyclient");
      expect(httpRoute.continue).not.toHaveBeenCalled();

      const socketHandler = context.routeWebSocket.mock.calls[0][1];
      const socketRoute = {
        url: () => "ws://169.254.169.254/private",
        close: vi.fn().mockResolvedValue(undefined),
        connectToServer: vi.fn(),
      };
      await socketHandler(socketRoute);
      expect(socketRoute.close).toHaveBeenCalledWith({ code: 1008, reason: "blocked target" });
      expect(socketRoute.connectToServer).not.toHaveBeenCalled();
    });
  });

  it.each(["initialization", "network guard", "page creation"])(
    "closes the context when %s fails before the callback",
    async (stage) => {
      const { context, page } = createContext();
      launch.mockResolvedValue({ newContext: vi.fn().mockResolvedValue(context) });
      const failure = new Error(`${stage} failed`);
      if (stage === "initialization") context.addInitScript.mockRejectedValueOnce(failure);
      else if (stage === "network guard") installPublicNetworkGuard.mockRejectedValueOnce(failure);
      else context.newPage.mockRejectedValueOnce(failure);
      const callback = vi.fn();
      const { BrowserManager } = await import("../manager.js");

      await expect(new BrowserManager().withUnprofiledPage(callback)).rejects.toBe(failure);

      expect(callback).not.toHaveBeenCalled();
      expect(page.close).not.toHaveBeenCalled();
      expect(context.close).toHaveBeenCalledTimes(1);
    },
  );
});
