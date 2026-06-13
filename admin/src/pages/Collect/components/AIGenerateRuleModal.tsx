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
import { TechnicalDetails } from '@/components/ui';
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
  Tooltip,
  Typography,
  message,
} from 'antd';
import type { ComponentType, CSSProperties } from 'react';
import { useEffect, useMemo, useState } from 'react';
import type { CollectRuleAIGenerateResult } from '@/services/collectRuleAI';
import { aiGenerateCollectRule } from '@/services/collectRuleAI';
import { createCollectRule } from '@/services/collectRules';
import { BrowserProfileLoginPanel } from '@/pages/Collect/components/BrowserProfileLoginPanel';
import { RuleQualityScoreCard } from '@/pages/Collect/components/RuleQualityScoreCard';
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
  { value: 'title', label: '商品标题', hint: '建议勾选', Icon: FontSizeOutlined },
  { value: 'price', label: '商品价格', hint: '建议勾选', Icon: DollarOutlined },
  { value: 'mainImages', label: '商品主图', hint: '建议勾选', Icon: PictureOutlined },
  { value: 'descriptionImages', label: '详情图片', hint: '建议勾选', Icon: FileImageOutlined },
  { value: 'attributes', label: '商品参数', hint: '规格参数', Icon: UnorderedListOutlined },
  {
    value: 'skus',
    label: '商品规格',
    hint: '高级 · 不一定能识别',
    advanced: true,
    Icon: TagsOutlined,
  },
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

  const canSaveEnabled = result?.qualityGate?.allowSaveEnabled === true;

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
      message.warning('请填写商品链接');
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
      if (res.qualityGate?.allowSaveEnabled) {
        message.success('采集规则已生成，识别效果达标，可保存并启用');
      } else {
        message.warning('规则已生成并完成测试，但识别效果未达标，建议重新生成或保存为草稿后手动调整');
      }
    } catch (e) {
      message.error(mapCollectErrorMessage(e));
    } finally {
      setGenerating(false);
    }
  };

  const handleSave = async (status: 'enabled' | 'disabled') => {
    if (status === 'enabled' && !canSaveEnabled) {
      message.warning('当前规则识别效果较差，建议重新生成或手动调整后再启用');
      return;
    }
    let parsed: unknown;
    try {
      parsed = JSON.parse(ruleJson || '{}');
    } catch {
      message.error('采集规则内容格式不正确');
      return;
    }
    const name = formName.trim() || result?.suggestedName;
    const domain = formDomain.trim() || result?.domain;
    if (!name || !domain) {
      message.warning('请填写规则名称与适用网站');
      return;
    }
    setSaving(true);
    try {
      await createCollectRule({
        name,
        domain,
        priority: formPriority,
        status,
        rule: parsed,
      });
      message.success(status === 'enabled' ? '采集规则已保存并启用' : '采集规则已保存为草稿（停用）');
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
          <span>AI 帮我生成规则</span>
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
                AI 帮我生成规则
              </Button>
            ) : (
              <>
                <Button loading={saving} onClick={() => void handleSave('disabled')}>
                  保存为草稿
                </Button>
                {canSaveEnabled ? (
                  <Button type="primary" loading={saving} onClick={() => void handleSave('enabled')}>
                    保存并启用
                  </Button>
                ) : (
                  <Tooltip title="识别效果未达标或缺少标题/主图规则，请先调整或重新生成">
                    <Button type="primary" disabled>
                      保存并启用
                    </Button>
                  </Tooltip>
                )}
              </>
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
            { title: '生成采集规则' },
            { title: '确认保存' },
          ]}
        />

        <Alert
          type="info"
          showIcon
          className="tm-ai-rule-modal__alert"
          message="输入一个商品链接，系统会先读取页面上的商品信息，再让 AI 帮你生成采集规则。生成后会自动测试标题、价格、图片等内容是否能识别。"
          description="商品规格、库存、实时价格通常由网站动态加载，不一定都能自动识别。识别评分低于 60 时不建议直接启用。"
        />
        <Typography.Paragraph type="secondary" className="tm-ai-rule-modal__hero-text" style={{ marginBottom: 0 }}>
          请先在
          <Typography.Link onClick={() => history.push('/settings/ai')}>设置 → AI 设置</Typography.Link>
          配置并测试 AI 后再使用本功能。
        </Typography.Paragraph>

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
            <Typography.Text className="tm-ai-rule-modal__label">商品链接</Typography.Text>
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
                <Typography.Text className="tm-ai-rule-modal__label">适用网站</Typography.Text>
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
            要采集的内容
          </Typography.Title>
          <div className="tm-ai-rule-field-grid">
            {TARGET_FIELD_METAS.map(({ value, label, hint, advanced, Icon }) => {
              const active = targetFields.includes(value);
              const card = (
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
              if (advanced && value === 'skus') {
                return (
                  <Tooltip
                    key={value}
                    title="不同网站的规格信息结构差异很大，AI 生成的规则不一定能完整识别。建议先测试后再使用。"
                  >
                    <span className="tm-ai-rule-field-card-wrap">{card}</span>
                  </Tooltip>
                );
              }
              return card;
            })}
          </div>
        </section>

        <section className="tm-ai-rule-modal__section tm-ai-rule-modal__section--profile">
          <div className="tm-ai-rule-modal__profile-head">
            <div>
              <Typography.Title level={5} className="tm-ai-rule-modal__section-title">
                使用已登录的采集浏览器
              </Typography.Title>
              <Typography.Text type="secondary" className="tm-ai-rule-modal__hint">
                商品页需要登录时可选；系统不保存账号密码
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
            <Typography.Text type="secondary">正在读取商品页并生成采集规则…</Typography.Text>
          </div>
        ) : null}

        {result ? (
          <section className="tm-ai-rule-modal__section tm-ai-rule-modal__section--result">
            <Typography.Title level={5} className="tm-ai-rule-modal__section-title">
              识别效果
            </Typography.Title>

            {result.qualityGate ? (
              <RuleQualityScoreCard gate={result.qualityGate} confidence={result.confidence} />
            ) : null}

            <div className="tm-ai-rule-modal__result-stack">
              {result.plannedHint ? (
                <Alert type="info" showIcon className="tm-ai-rule-modal__alert" message={result.plannedHint} />
              ) : null}

              {!canSaveEnabled ? (
                <Alert
                  type="warning"
                  showIcon
                  className="tm-ai-rule-modal__alert"
                  message="当前规则识别效果较差，建议重新生成或手动调整后再启用"
                  description={
                    result.qualityGate?.blockReasons?.length ? (
                      <ul className="tm-ai-rule-modal__warn-list">
                        {result.qualityGate.blockReasons.map((r) => (
                          <li key={r}>{r}</li>
                        ))}
                      </ul>
                    ) : undefined
                  }
                />
              ) : null}

              {result.qualityGate?.suggestions?.length ? (
                <Alert
                  type="info"
                  showIcon
                  className="tm-ai-rule-modal__alert"
                  message="建议操作"
                  description={result.qualityGate.suggestions.join(' · ')}
                />
              ) : null}

              {result.missingGeneratedFields?.length ? (
                <Alert
                  type="warning"
                  showIcon
                  className="tm-ai-rule-modal__alert"
                  message="未能生成的字段"
                  description={result.missingGeneratedFields.join('、')}
                />
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
                      {result.warnings.slice(0, 8).map((w) => (
                        <li key={w}>{w}</li>
                      ))}
                    </ul>
                  }
                />
              ) : null}
            </div>

            {result.testResult ? (
              <RuleTestResultPanel result={result.testResult} showProduct={false} compact />
            ) : null}

            <TechnicalDetails label="采集规则内容（高级）">
              <Typography.Paragraph type="secondary" style={{ fontSize: 12, marginBottom: 8 }}>
                下方为采集规则 JSON，一般无需修改；格式错误会导致采集失败。
              </Typography.Paragraph>
              <ProFormTextArea
                label="规则 JSON"
                fieldProps={{
                  rows: 10,
                  value: ruleJson,
                  onChange: (e) => setRuleJson(e.target.value),
                  className: 'tm-ai-rule-modal__json',
                  style: { fontFamily: 'monospace', fontSize: 12 },
                }}
              />
            </TechnicalDetails>
          </section>
        ) : null}
      </div>
    </ModalForm>
  );
}
