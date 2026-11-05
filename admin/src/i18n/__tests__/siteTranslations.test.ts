import { describe, expect, it } from 'vitest';
import { hasTranslation, supportedLanguages, translateMessage } from '../dictionaries';
import { siteTranslations } from '../siteTranslations';

const placeholders = (value: string) => [...value.matchAll(/\{\w+\}/g)].map(([match]) => match).sort();

describe('shared site translations', () => {
  it('localizes the dashboard network fallback in every target language', () => {
    for (const language of supportedLanguages.slice(1)) {
      expect(hasTranslation(language, '网络或服务异常')).toBe(true);
      expect(translateMessage(language, '网络或服务异常')).not.toBe('网络或服务异常');
    }
  });

  it('provides complete non-empty values with matching placeholders', () => {
    for (const [source, values] of Object.entries(siteTranslations)) {
      expect(values, source).toHaveLength(5);
      for (const value of values) {
        expect(value.trim(), source).not.toBe('');
        expect(placeholders(value), source).toEqual(placeholders(source));
      }
    }
  });

  it('registers every shared site key in the runtime dictionary', () => {
    for (const [source, values] of Object.entries(siteTranslations)) {
      for (const language of supportedLanguages) {
        if (language === 'zh-CN') continue;
        const index = supportedLanguages.indexOf(language) - 1;
        expect(hasTranslation(language, source), `${language}: ${source}`).toBe(true);
        expect(translateMessage(language, source)).toBe(values[index]);
      }
    }
  });

  it('provides translations for the public homepage copy in every target language', () => {
    const homepageCopy = [
      '多来源商品采集',
      '统一承接商品链接与采集任务，让原始商品信息有序进入草稿流程。',
      'AI 商品运营',
      '围绕标题、描述与图片生成优化建议，并保留清晰的人工复核入口。',
      '多平台刊登协同',
      '通过标准化接口扩展平台能力，集中管理刊登草稿、任务与异常处理。',
      '订单与库存协同',
      '连接店铺、订单、规格匹配与库存同步，覆盖跨境 ERP 的关键闭环。',
      '私有化部署',
      '基于 PostgreSQL、Redis 与 Docker Compose，适合团队自部署和二次开发。',
      '安全的人机协作',
      '敏感配置加密保存，关键发布与客服外发动作保留人工确认。',
      '提交来源链接，创建可追踪的采集任务。',
      '统一维护商品、规格、图片与平台信息。',
      '生成标题、描述与图片处理建议。',
      '确认内容、映射与发布前检查结果。',
      '创建平台任务并跟踪结果与异常。',
      'TradeMind 聚焦商品采集、草稿整理、AI 内容优化、图片处理、多平台刊登与库存协同，为跨境团队提供一套可部署、可扩展的运营工作台。',
      '用清晰的任务、草稿与状态承接跨境商品运营，不把平台扩展成难以维护的重型 ERP。',
      '为自部署与二次开发而设计',
      '保留数据控制权，也保留持续扩展的空间',
      '前后端、采集服务、PostgreSQL 与 Redis 统一维护；AI、图片、存储、平台和采集能力通过标准化接口接入。',
      '登录',
      '免费注册',
      '登录工作台',
      '可复核',
      '可追踪',
      '运营流程',
      '创建账号',
      '查看开源项目',
    ];

    for (const source of homepageCopy) {
      for (const language of supportedLanguages.slice(1)) {
        expect(hasTranslation(language, source), `${language}: ${source}`).toBe(true);
      }
    }
  });

  it('provides translations for inventory sync batch labels in every target language', () => {
    const batchCopy = [
      '批量创建的库存同步任务会归入批次；后台任务进程完成后按任务聚合回写批次统计。单次创建上限见',
      '中的单次创建上限',
      '批次列表',
      '打开任务列表（按批次筛选）',
    ];

    for (const source of batchCopy) {
      for (const language of supportedLanguages.slice(1)) {
        expect(hasTranslation(language, source), `${language}: ${source}`).toBe(true);
      }
    }
  });
});
