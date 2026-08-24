import { expect, type Locator, type Page } from '@playwright/test';
import type { ConsoleGuard } from './console-guard';
import type { NetworkWriteGuard } from './network-guard';

export async function expectNoRootOverflow(page: Page) {
  const value = await page.evaluate(() => ({
    htmlScrollWidth: document.documentElement.scrollWidth,
    htmlClientWidth: document.documentElement.clientWidth,
    bodyScrollWidth: document.body.scrollWidth,
    bodyClientWidth: document.body.clientWidth,
  }));
  expect(value.htmlScrollWidth, `html overflow: ${JSON.stringify(value)}`).toBeLessThanOrEqual(value.htmlClientWidth + 1);
  expect(value.bodyScrollWidth, `body overflow: ${JSON.stringify(value)}`).toBeLessThanOrEqual(value.bodyClientWidth + 1);
}

export async function expectActiveTab(page: Page, tabName: string) {
  const tab = page.getByRole('tab', { name: new RegExp(tabName) });
  await expect(tab, `active tab ${tabName}`).toHaveAttribute('aria-selected', 'true');
}

export async function expectSectionVisible(page: Page, sectionId: string) {
  await expect(page.locator(`#${sectionId}`), `section ${sectionId}`).toBeVisible();
}

export async function expectModalWithinViewport(page: Page) {
  await expectOverlayWithinViewport(page.locator('.ant-modal:visible').first(), page, 'modal');
}

export async function expectDrawerWithinViewport(page: Page) {
  await expectOverlayWithinViewport(page.locator('.ant-drawer-content-wrapper:visible').first(), page, 'drawer');
}

export async function expectHeaderContentAligned(page: Page) {
  const value = await page.evaluate(() => {
    const header = document.querySelector('.tm-page-container .ant-page-header')?.getBoundingClientRect();
    const content = document.querySelector('.tm-page-container__content')?.getBoundingClientRect();
    if (!header || !content) return null;
    return {
      headerLeft: header.left,
      headerRight: header.right,
      contentLeft: content.left,
      contentRight: content.right,
      leftDelta: Math.abs(header.left - content.left),
      rightDelta: Math.abs(header.right - content.right),
    };
  });
  expect(value, 'header/content alignment metrics').not.toBeNull();
  if (!value) return;
  expect(value.leftDelta, `header/content left delta ${JSON.stringify(value)}`).toBeLessThanOrEqual(4);
  expect(value.rightDelta, `header/content right delta ${JSON.stringify(value)}`).toBeLessThanOrEqual(4);
}

export async function expectHeaderActionsSpaced(page: Page) {
  const value = await page.locator('.tm-page-header-extra').first().evaluate((element) => {
    const style = window.getComputedStyle(element);
    return {
      display: style.display,
      columnGap: Number.parseFloat(style.columnGap || '0'),
      rowGap: Number.parseFloat(style.rowGap || '0'),
    };
  });
  expect(value.display, `header action layout ${JSON.stringify(value)}`).toMatch(/flex$/);
  expect(value.columnGap, `header action column gap ${JSON.stringify(value)}`).toBeGreaterThanOrEqual(8);
  expect(value.rowGap, `header action row gap ${JSON.stringify(value)}`).toBeGreaterThanOrEqual(8);
}

export async function expectPageContentGuttersWithin(page: Page, maxGutter: number) {
  const value = await page.evaluate(() => {
    const shell = document.querySelector('.ant-pro-layout-content')?.getBoundingClientRect();
    const contentElement = document.querySelector<HTMLElement>('.tm-page-container__content');
    const content = contentElement?.getBoundingClientRect();
    if (!shell || !content || !contentElement) return null;
    const style = window.getComputedStyle(contentElement);
    return {
      shellLeft: shell.left,
      shellRight: shell.right,
      contentLeft: content.left,
      contentRight: content.right,
      leftGutter: content.left - shell.left + Number.parseFloat(style.paddingLeft || '0'),
      rightGutter: shell.right - content.right + Number.parseFloat(style.paddingRight || '0'),
    };
  });
  expect(value, 'page content gutter metrics').not.toBeNull();
  if (!value) return;
  expect(value.leftGutter, `page left gutter ${JSON.stringify(value)}`).toBeGreaterThanOrEqual(-1);
  expect(value.rightGutter, `page right gutter ${JSON.stringify(value)}`).toBeGreaterThanOrEqual(-1);
  expect(value.leftGutter, `page left gutter ${JSON.stringify(value)}`).toBeLessThanOrEqual(maxGutter + 1);
  expect(value.rightGutter, `page right gutter ${JSON.stringify(value)}`).toBeLessThanOrEqual(maxGutter + 1);
}

export async function expectAccountInTopNavbar(page: Page) {
  const navbar = page.getByRole('navigation', { name: '内容导航栏' });
  const account = navbar.getByRole('button', { name: /^当前用户 / });

  await expect(navbar).toBeVisible();
  await expect(account).toBeVisible();
  await expect(page.getByRole('button', { name: /^当前用户 / })).toHaveCount(1);
  await expect(
    page.locator('.ant-pro-sider').getByRole('button', { name: /^当前用户 / }),
  ).toHaveCount(0);

  const value = await page.evaluate(() => {
    const navbarRect = document.querySelector('.tm-app-top-nav')?.getBoundingClientRect();
    const accountRect = document.querySelector('.tm-app-top-nav__user')?.getBoundingClientRect();
    const brandRect = Array.from(document.querySelectorAll<HTMLElement>('.ant-pro-sider-logo'))
      .map((element) => element.getBoundingClientRect())
      .find(
        (rect) =>
          rect.width > 0 &&
          rect.height > 0 &&
          rect.right > 0 &&
          rect.left < window.innerWidth &&
          rect.bottom > 0,
      );
    const mobileHeaderRect = document
      .querySelector<HTMLElement>('.ant-pro-layout-header')
      ?.getBoundingClientRect();
    const referenceRect = brandRect ?? mobileHeaderRect;
    if (!navbarRect || !accountRect || !mobileHeaderRect) return null;
    return {
      navbarLeft: navbarRect.left,
      navbarRight: navbarRect.right,
      navbarTop: navbarRect.top,
      navbarBottom: navbarRect.bottom,
      navbarHeight: navbarRect.height,
      accountLeft: accountRect.left,
      accountRight: accountRect.right,
      headerTop: mobileHeaderRect.top,
      headerBottom: mobileHeaderRect.bottom,
      referenceHeight: referenceRect?.height ?? null,
    };
  });

  expect(value, 'top navbar account metrics').not.toBeNull();
  if (!value) return;
  expect(value.accountLeft, `account inside navbar ${JSON.stringify(value)}`).toBeGreaterThanOrEqual(
    value.navbarLeft,
  );
  expect(value.accountRight, `account inside navbar ${JSON.stringify(value)}`).toBeLessThanOrEqual(
    value.navbarRight + 1,
  );
  expect(value.navbarTop, `navbar inside header ${JSON.stringify(value)}`).toBeGreaterThanOrEqual(
    value.headerTop - 1,
  );
  expect(
    value.navbarBottom,
    `navbar inside header ${JSON.stringify(value)}`,
  ).toBeLessThanOrEqual(value.headerBottom + 1);
  expect(value.referenceHeight, `brand/header height ${JSON.stringify(value)}`).not.toBeNull();
  if (value.referenceHeight === null) return;
  expect(
    Math.abs(value.navbarHeight - value.referenceHeight),
    `navbar matches brand/header height ${JSON.stringify(value)}`,
  ).toBeLessThanOrEqual(1);
}

export async function expectTopNavbarScrollBehavior(page: Page) {
  const navbar = page.getByRole('navigation', { name: '内容导航栏' });
  const layoutHeader = page.locator('.ant-pro-layout-header').first();

  await expect(layoutHeader).toHaveCSS('position', 'fixed');
  const before = await page.evaluate(() => {
    const header = document.querySelector<HTMLElement>('.ant-pro-layout-header');
    const navigation = document.querySelector<HTMLElement>('.tm-app-top-nav');
    if (!header || !navigation) return null;
    return {
      headerTop: header.getBoundingClientRect().top,
      navigationTop: navigation.getBoundingClientRect().top,
    };
  });
  expect(before, 'fixed header/navigation positions before scroll').not.toBeNull();

  const maxScroll = await page.evaluate(() => {
    const scroller = document.scrollingElement as HTMLElement | null;
    return scroller ? scroller.scrollHeight - scroller.clientHeight : 0;
  });
  expect(maxScroll, 'page has enough content to exercise sticky navigation').toBeGreaterThan(0);

  await page.evaluate(() => {
    const scroller = document.scrollingElement as HTMLElement | null;
    if (scroller) scroller.scrollTop = Math.min(120, scroller.scrollHeight - scroller.clientHeight);
  });

  await expect
    .poll(() =>
      page.evaluate(() => {
        const scroller = document.scrollingElement as HTMLElement | null;
        return scroller?.scrollTop ?? 0;
      }),
    )
    .toBeGreaterThan(0);

  const positions = await page.evaluate(() => {
    const header = document.querySelector<HTMLElement>('.ant-pro-layout-header');
    const navigation = document.querySelector<HTMLElement>('.tm-app-top-nav');
    if (!header || !navigation) return null;
    return {
      headerTop: header.getBoundingClientRect().top,
      navigationTop: navigation.getBoundingClientRect().top,
    };
  });
  expect(positions, 'fixed header/navigation positions after scroll').not.toBeNull();
  if (before && positions) {
    expect(
      Math.abs(positions.headerTop - before.headerTop),
      `fixed header after scroll ${JSON.stringify({ before, positions })}`,
    ).toBeLessThanOrEqual(1);
    expect(
      Math.abs(positions.navigationTop - before.navigationTop),
      `navbar remains in fixed header ${JSON.stringify({ before, positions })}`,
    ).toBeLessThanOrEqual(1);
  }

  await page.evaluate(() => {
    const scroller = document.scrollingElement as HTMLElement | null;
    if (scroller) scroller.scrollTop = 0;
  });
  await expect(navbar).toBeVisible();
}

export async function expectPageChromeScrollbarsHidden(page: Page) {
  const value = await page.evaluate(() => {
    const layout = document.querySelector('.tm-app-layout');
    const sider = layout?.querySelector('.ant-pro-sider');
    const siderScroller = sider
      ? [sider, ...Array.from(sider.querySelectorAll('*'))].find((element) => {
          const overflowY = window.getComputedStyle(element).overflowY;
          return overflowY === 'auto' || overflowY === 'scroll';
        })
      : undefined;
    const pageScroller = document.scrollingElement as HTMLElement | null;
    const pageMaxScroll = pageScroller ? pageScroller.scrollHeight - pageScroller.clientHeight : 0;
    const pageScrollTopBefore = pageScroller?.scrollTop ?? 0;
    if (pageScroller && pageMaxScroll > 0) {
      pageScroller.scrollTop = Math.min(pageScrollTopBefore + 80, pageMaxScroll);
    }
    const pageScrollTopAfter = pageScroller?.scrollTop ?? 0;
    if (pageScroller) pageScroller.scrollTop = pageScrollTopBefore;

    const scrollbarState = (element: Element) => {
      const scrollbarWidth = window.getComputedStyle(element).getPropertyValue('scrollbar-width');
      const webkitDisplay = window.getComputedStyle(element, '::-webkit-scrollbar').display;

      return {
        scrollbarWidth,
        webkitDisplay,
        hidden: scrollbarWidth === 'none' || webkitDisplay === 'none',
      };
    };

    return {
      hasLayout: Boolean(layout),
      hasSiderScroller: Boolean(siderScroller),
      siderOverflowY: siderScroller ? window.getComputedStyle(siderScroller).overflowY : null,
      html: scrollbarState(document.documentElement),
      body: scrollbarState(document.body),
      sider: siderScroller ? scrollbarState(siderScroller) : null,
      pageMaxScroll,
      pageScrollTopBefore,
      pageScrollTopAfter,
    };
  });

  expect(value.hasLayout, `app layout ${JSON.stringify(value)}`).toBe(true);
  expect(value.hasSiderScroller, `sider scroll container ${JSON.stringify(value)}`).toBe(true);
  expect(value.siderOverflowY, `sider keeps scrolling ${JSON.stringify(value)}`).toBe('auto');
  expect(value.html.hidden, `html scrollbar ${JSON.stringify(value)}`).toBe(true);
  expect(value.body.hidden, `body scrollbar ${JSON.stringify(value)}`).toBe(true);
  expect(value.sider?.hidden, `sider scrollbar ${JSON.stringify(value)}`).toBe(true);
  expect(value.pageMaxScroll, `page remains scrollable ${JSON.stringify(value)}`).toBeGreaterThan(0);
  expect(value.pageScrollTopAfter, `page scroll movement ${JSON.stringify(value)}`).toBeGreaterThan(
    value.pageScrollTopBefore,
  );
}

export async function expectRequestCount(tracker: NetworkWriteGuard, operation: string, count: number) {
  await tracker.expectRequestCount(operation, count);
}

export async function expectNoUnexpectedWrites(tracker: NetworkWriteGuard) {
  await tracker.expectNoUnexpectedWrites();
}

export async function expectNoFatalConsoleErrors(consoleGuard: ConsoleGuard) {
  await consoleGuard.expectNoFatalErrors();
}

async function expectOverlayWithinViewport(locator: Locator, page: Page, label: string) {
  await expect(locator, `${label} visible`).toBeVisible();
  const box = await locator.boundingBox();
  const viewport = page.viewportSize();
  expect(box, `${label} bounding box`).not.toBeNull();
  expect(viewport, 'viewport size').not.toBeNull();
  if (!box || !viewport) return;
  expect(box.x, `${label} left`).toBeGreaterThanOrEqual(0);
  expect(box.y, `${label} top`).toBeGreaterThanOrEqual(0);
  expect(box.x + box.width, `${label} right`).toBeLessThanOrEqual(viewport.width + 1);
  expect(box.y + box.height, `${label} bottom`).toBeLessThanOrEqual(viewport.height + 1);
}
