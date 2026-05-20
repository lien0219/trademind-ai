const PDD_HOSTS = new Set([
  'yangkeduo.com',
  'mobile.yangkeduo.com',
  'pinduoduo.com',
  'mobile.pinduoduo.com',
]);

export function isPinduoduoHost(hostname: string): boolean {
  const h = hostname.trim().toLowerCase();
  if (PDD_HOSTS.has(h)) return true;
  return h.endsWith('.yangkeduo.com') || h.endsWith('.pinduoduo.com');
}

/** 语义校验：http(s) + 拼多多域名 + 商品详情路径或 goods_id 参数 */
export function validatePinduoduoUrl(urlStr: string): boolean {
  try {
    const u = new URL(urlStr.trim());
    if (u.protocol !== 'http:' && u.protocol !== 'https:') return false;
    if (!isPinduoduoHost(u.hostname)) return false;
    const path = u.pathname.toLowerCase();
    const hasGoodsId = /(?:^|[?&])goods_id=\d+/i.test(u.search);
    const isGoodsPath =
      path.includes('goods') ||
      path.includes('goods_detail') ||
      path.includes('comm_goods') ||
      hasGoodsId;
    return isGoodsPath;
  } catch {
    return false;
  }
}

export function normalizePinduoduoNavUrl(raw: string): string {
  try {
    const u = new URL(raw.trim());
    const goodsId = u.searchParams.get('goods_id') ?? u.searchParams.get('goodsId');
    if (goodsId && /^\d+$/.test(goodsId)) {
      return `https://mobile.yangkeduo.com/goods.html?goods_id=${goodsId}`;
    }
  } catch {
    /* keep raw */
  }
  return raw.trim();
}
