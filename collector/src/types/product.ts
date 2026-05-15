/**
 * 与 Go / 主业务约定的统一商品结构（任何采集源最终归一为此格式）。
 */
export type NormalizedProduct = {
  source: string;
  sourceUrl: string;
  title: string;
  currency: string;
  mainImages: string[];
  descriptionImages: string[];
  attributes: Record<string, string | number | boolean>;
  skus: ProductSku[];
  /** 平台页原始快照，必填以便复盘与二次解析 */
  raw: Record<string, unknown>;
};

export type ProductSku = {
  id?: string;
  /** 如颜色、尺码等键值 */
  properties?: Record<string, string>;
  price?: number;
  stock?: number;
  skuCode?: string;
  image?: string;
  /** SKU 粒度原始快照（Go 入库时保留在 product_skus.raw_data） */
  raw?: Record<string, unknown>;
};
