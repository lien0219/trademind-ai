import { history } from '@umijs/max';
import {
  DollarOutlined,
  FileImageOutlined,
  FontSizeOutlined,
  LinkOutlined,
  PictureOutlined,
  RobotOutlined,
  TagsOutlined,
  UnorderedListOutlined,
} from '@ant-design/icons';
import { ModalForm, ProFormTextArea } from '@ant-design/pro-components';
import {
  Alert,
  Button,
  Col,
  Input,
  InputNumber,
  Row,
  Space,
  Spin,
  Steps,
  Switch,
  Tag,
  Typography,
  message,
} from 'antd';
import type { ComponentType, CSSProperties } from 'react';
import { useEffect, useMemo, useState } from 'react';
import type { CollectRuleAIGenerateResult } from '@/services/collectRuleAI';
import { aiGenerateCollectRule } from '@/services/collectRuleAI';
import { createCollectRule } from '@/services/collectRules';
import { BrowserProfileLoginPanel } from '@/pages/Collect/components/BrowserProfileLoginPanel';
import { RuleTestResultPanel } from '@/pages/Collect/components/RuleTestResultPanel';
import { mapCollectErrorMessage } from '@/constants/collectErrors';
import { detectCustomCollectPlatform } from '@/utils/customCollectPlatform';

type TargetFieldMeta = {
  value: string;
  label: string;
  hint: string;
  advanced?: boolean;
  Icon: ComponentType<{ style?: CSSProperties }>;
};

const TARGET_FIELD_METAS: TargetFieldMeta[] = [
  { value: 'title', label: '标题', hint: '必填字段', Icon: FontSizeOutlined },
  { value: 'price', label: '价格', hint: '文本或 meta', Icon: DollarOutlined },
  { value: 'mainImages', label: '主图', hint: '画廊 / OG 图', Icon: PictureOutlined },
  { value: 'descriptionImages', label: '详情图', hint: '详情区 img', Icon: FileImageOutlined },
  { value: 'attributes', label: '属性', hint: '规格参数对', Icon: UnorderedListOutlined },
  { value: 'skus', label: 'SKU', hint: '高级 · 不稳定', advanced: true, Icon: TagsOutlined },
];

type Props = {
  open: boolean;
  onClose: () => void;
  initialUrl?: string;
  onSaved?: () => void;
};

function suggestDomainFromUrl(url: string): string {
  try {
    const host = new URL(url.trim()).hostname.toLowerCase();
    const parts = host.split('.');
    if (parts.length >= 2) return parts.slice(-2).join('.');
    return host;
  } catch {
    return '';
  }
}

function toggleField(list: string[], value: string): string[] {
  return list.includes(value) ? list.filter((v) => v !== value) : [...list, value];
}

export function AIGenerateRuleModal({ open, onClose, initialUrl, onSaved }: Props) {
  const [generating, setGenerating] = useState(false);
  const [saving, setSaving] = useState(false);
  const [result, setResult] = useState<CollectRuleAIGenerateResult | null>(null);
  const [ruleJson, setRuleJson] = useState('');
  const [formUrl, setFormUrl] = useState('');
  const [formDomain, setFormDomain] = useState('');
  const [formName, setFormName] = useState('');
  const [formPriority, setFormPriority] = useState(100);
  const [profileId, setProfileId] = useState<string | undefined>();
  const [useBrowserProfile, setUseBrowserProfile] = useState(false);
  const [targetFields, setTargetFields] = useState<string[]>([
    'title',
    'price',
    'mainImages',
    'descriptionImages',
    'attributes',
  ]);

  const platformHint = useMemo(() => {
    const url = formUrl.trim();
    if (!url) return null;
    return detectCustomCollectPlatform(url);
  }, [formUrl]);

  const stepIndex = result ? 2 : generating ? 1 : 0;
  const generateDisabled = platformHint?.kind === 'blocked' || !formUrl.trim() || generating;

  useEffect(() => {
    if (!open) {
      setResult(null);
      setRuleJson('');
      setGenerating(false);
      setSaving(false);
      setProfileId(undefined);
      setUseBrowserProfile(false);
      setFormUrl('');
      setFormDomain('');
      setFormName('');
      setFormPriority(100);
      setTargetFields(['title', 'price', 'mainImages', 'descriptionImages', 'attributes']);
      return;
    }
    const url = initialUrl?.trim() ?? '';
    if (url) {
      setFormUrl(url);
      setFormDomain(suggestDomainFromUrl(url));
    }
  }, [open, initialUrl]);

  const runGenerate = async () => {
    const url = formUrl.trim();
    if (!url) {
      message.warning('请填写商品 URL');
      return;
    }
    if (platformHint?.kind === 'blocked') {
      message.warning(platformHint.message);
      return;
    }
    setGenerating(true);
    setResult(null);
    try {
      const domain = formDomain.trim() || suggestDomainFromUrl(url);
      const res = await aiGenerateCollectRule({
        url,
        domain,
        profileId: useBrowserProfile ? profileId : undefined,
        useBrowserProfile: useBrowserProfile && Boolean(profileId),
        targetFields,
        ruleName: formName.trim() || undefined,
      });
      setResult(res);
      setRuleJson(JSON.stringify(res.rule ?? {}, null, 2));
      setFormDomain(res.domain || domain);
      if (!formName.trim() && res.suggestedName) {
        setFormName(res.suggestedName);
      }
      message.success('规则已生成并完成测试');
    } catch (e) {
      message.error(mapCollectErrorMessage(e));
    } finally {
      setGenerating(false);
    }
  };

  const handleSave = async () => {
    let parsed: unknown;
    try {
      parsed = JSON.parse(ruleJson || '{}');
    } catch {
      message.error('rule JSON 格式不正确');
      return;
    }
    const name = formName.trim() || result?.suggestedName;
    const domain = formDomain.trim() || result?.domain;
    if (!name || !domain) {
      message.warning('请填写规则名称与域名');
      return;
    }
    setSaving(true);
    try {
      await createCollectRule({
        name,
        domain,
        priority: formPriority,
        status: 'enabled',
        rule: parsed,
      });
      message.success('规则已保存');
      onSaved?.();
      onClose();
    } catch (e) {
      message.error(mapCollectErrorMessage(e));
    } finally {
      setSaving(false);
    }
  };

  return (
    <ModalForm
      title={
        <Space size={8}>
          <RobotOutlined style={{ color: 'var(--ant-color-primary)' }} />
          <span>AI 生成采集规则</span>
        </Space>
      }
      open={open}
      className="tm-ai-rule-modal"
      modalProps={{
        destroyOnHidden: true,
        width: 800,
        onCancel: onClose,
        styles: { body: { paddingTop: 12 } },
      }}
      submitter={{
        render: () => (
          <Space wrap className="tm-action-space tm-ai-rule-modal__footer">
            <Button onClick={onClose}>关闭</Button>
            {result ? (
              <Button loading={generating} disabled={generateDisabled} onClick={() => void runGenerate()}>
                重新生成
              </Button>
            ) : null}
            {!result ? (
              <Button type="primary" loading={generating} disabled={generateDisabled} onClick={() => void runGenerate()}>
                分析页面并生成规则
              </Button>
            ) : (
              <Button type="primary" loading={saving} onClick={() => void handleSave()}>
                保存规则
              </Button>
            )}
          </Space>
        ),
      }}
      onOpenChange={(v) => {
        if (!v) onClose();
      }}
      onFinish={async () => false}
    >
      <div className="tm-ai-rule-modal__body">
        <Steps
          size="small"
          current={stepIndex}
          className="tm-ai-rule-modal__steps"
          items={[
            { title: '填写信息' },
            { title: 'AI 分析生成' },
            { title: '确认保存' },
          ]}
        />

        <div className="tm-ai-rule-modal__hero">
          <Typography.Text type="secondary" className="tm-ai-rule-modal__hero-text">
            请先在
            <Typography.Link onClick={() => history.push('/settings/ai')}>设置 → AI 设置</Typography.Link>
            配置并测试 AI。系统仅分析页面结构摘要（不含完整 HTML），生成后自动执行规则测试。
          </Typography.Text>
        </div>

        {platformHint?.kind === 'blocked' ? (
          <Alert type="warning" showIcon className="tm-ai-rule-modal__alert" message={platformHint.message} />
        ) : null}
        {platformHint?.kind === 'planned' ? (
          <Alert type="info" showIcon className="tm-ai-rule-modal__alert" message={platformHint.message} />
        ) : null}

        <section className="tm-ai-rule-modal__section">
          <Typography.Title level={5} className="tm-ai-rule-modal__section-title">
            商品与规则
          </Typography.Title>
          <div className="tm-ai-rule-modal__field">
            <Typography.Text className="tm-ai-rule-modal__label">商品 URL</Typography.Text>
            <Input
              prefix={<LinkOutlined style={{ color: 'var(--ant-color-text-quaternary)' }} />}
              placeholder="https://example.com/product/..."
              value={formUrl}
              onChange={(e) => {
                setFormUrl(e.target.value);
                setResult(null);
                setFormDomain(suggestDomainFromUrl(e.target.value));
              }}
            />
          </div>
          <Row gutter={12}>
            <Col xs={24} sm={12}>
              <div className="tm-ai-rule-modal__field">
                <Typography.Text className="tm-ai-rule-modal__label">规则域名</Typography.Text>
                <Input
                  placeholder="jd.com"
                  value={formDomain}
                  onChange={(e) => setFormDomain(e.target.value)}
                />
                <Typography.Text type="secondary" className="tm-ai-rule-modal__hint">
                  仅填主机名
                </Typography.Text>
              </div>
            </Col>
            <Col xs={24} sm={12}>
              <div className="tm-ai-rule-modal__field">
                <Typography.Text className="tm-ai-rule-modal__label">规则名称</Typography.Text>
                <Input placeholder="例如：京东-自定义" value={formName} onChange={(e) => setFormName(e.target.value)} />
              </div>
            </Col>
          </Row>
          <div className="tm-ai-rule-modal__field tm-ai-rule-modal__field--inline">
            <Typography.Text className="tm-ai-rule-modal__label">优先级</Typography.Text>
            <InputNumber min={0} max={1_000_000} value={formPriority} onChange={(v) => setFormPriority(typeof v === 'number' ? v : 100)} />
          </div>
        </section>

        <section className="tm-ai-rule-modal__section">
          <Typography.Title level={5} className="tm-ai-rule-modal__section-title">
            目标字段
          </Typography.Title>
          <div className="tm-ai-rule-field-grid">
            {TARGET_FIELD_METAS.map(({ value, label, hint, advanced, Icon }) => {
              const active = targetFields.includes(value);
              return (
                <button
                  key={value}
                  type="button"
                  className={`tm-ai-rule-field-card${active ? ' is-active' : ''}${advanced ? ' is-advanced' : ''}`}
                  onClick={() => {
                    setTargetFields((prev) => toggleField(prev, value));
                    setResult(null);
                  }}
                >
                  <span className="tm-ai-rule-field-card__icon">
                    <Icon />
                  </span>
                  <span className="tm-ai-rule-field-card__text">
                    <span className="tm-ai-rule-field-card__title">
                      {label}
                      {advanced ? <Tag className="tm-ai-rule-field-card__tag">高级</Tag> : null}
                    </span>
                    <span className="tm-ai-rule-field-card__hint">{hint}</span>
                  </span>
                </button>
              );
            })}
          </div>
        </section>

        <section className="tm-ai-rule-modal__section tm-ai-rule-modal__section--profile">
          <div className="tm-ai-rule-modal__profile-head">
            <div>
              <Typography.Title level={5} className="tm-ai-rule-modal__section-title">
                浏览器 Profile
              </Typography.Title>
              <Typography.Text type="secondary" className="tm-ai-rule-modal__hint">
                登录态页面可选；系统不保存账号密码
              </Typography.Text>
            </div>
            <Switch checked={useBrowserProfile} onChange={setUseBrowserProfile} />
          </div>
          {useBrowserProfile ? (
            <div className="tm-ai-rule-modal__profile-panel">
              <BrowserProfileLoginPanel
                tone="optional"
                url={formUrl}
                domain={formDomain}
                profileId={profileId}
                useBrowserProfile={useBrowserProfile}
                onProfileIdChange={setProfileId}
                onUseProfileChange={setUseBrowserProfile}
              />
            </div>
          ) : null}
        </section>

        {generating ? (
          <div className="tm-ai-rule-modal__loading">
            <Spin size="large" />
            <Typography.Text type="secondary">正在分析页面结构并调用 AI 生成规则…</Typography.Text>
          </div>
        ) : null}

        {result ? (
          <section className="tm-ai-rule-modal__section tm-ai-rule-modal__section--result">
            <Typography.Title level={5} className="tm-ai-rule-modal__section-title">
              生成结果
              {typeof result.confidence === 'number' ? (
                <Tag color="processing" className="tm-ai-rule-modal__confidence">
                  置信度 {Math.round(result.confidence * 100)}%
                </Tag>
              ) : null}
            </Typography.Title>

            {result.plannedHint ? (
              <Alert type="info" showIcon className="tm-ai-rule-modal__alert" message={result.plannedHint} />
            ) : null}
            {result.explanation ? (
              <div className="tm-ai-rule-modal__explain">{result.explanation}</div>
            ) : null}
            {result.warnings?.length ? (
              <Alert
                type="warning"
                showIcon
                className="tm-ai-rule-modal__alert"
                message="注意事项"
                description={
                  <ul className="tm-ai-rule-modal__warn-list">
                    {result.warnings.map((w) => (
                      <li key={w}>{w}</li>
                    ))}
                  </ul>
                }
              />
            ) : null}

            <ProFormTextArea
              label="rule JSON（可编辑）"
              fieldProps={{
                rows: 10,
                value: ruleJson,
                onChange: (e) => setRuleJson(e.target.value),
                className: 'tm-ai-rule-modal__json',
              }}
            />
            {result.testResult ? <RuleTestResultPanel result={result.testResult} /> : null}
          </section>
        ) : null}
      </div>
    </ModalForm>
  );
}
