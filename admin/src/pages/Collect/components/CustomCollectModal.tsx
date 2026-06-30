import { ModalForm, ProFormSelect, ProFormText } from '@ant-design/pro-components';
import { history } from '@umijs/max';
import { Alert, Button, message, Space, Spin } from 'antd';
import { useEffect, useMemo, useState } from 'react';
import { createCollectTask } from '@/services/collectTasks';
import type { CollectRuleRow, CollectRuleTestResult } from '@/services/collectRules';
import { queryCollectRules, testCollectRule } from '@/services/collectRules';
import {
  formatRuleDomainMismatchMessage,
  ruleMatchesURL,
  suggestRuleDomainForHost,
} from '@/utils/collectRuleMatch';
import { COLLECT_SUCCESS_SHOP_HINT } from '@/constants/copywriting';
import { mapCollectErrorMessage } from '@/constants/collectErrors';
import { BrowserProfileLoginPanel } from '@/pages/Collect/components/BrowserProfileLoginPanel';
import { AIGenerateRuleModal } from '@/pages/Collect/components/AIGenerateRuleModal';
import { RuleTestResultPanel } from '@/pages/Collect/components/RuleTestResultPanel';
import {
  CUSTOM_COLLECT_USAGE_LINES,
  detectCustomCollectPlatform,
  type CustomCollectPlatformHint,
} from '@/utils/customCollectPlatform';
import { NO_COLLECT_RULE_MESSAGE } from '@/utils/collectProviderStatus';

type Props = {
  open: boolean;
  onClose: () => void;
};

function resolveRuleId(
  rules: CollectRuleRow[],
  url: string,
  ruleId?: string,
): { ok: true; id: string } | { ok: false; message: string } {
  const rid = ruleId?.trim();
  if (rid) {
    const rule = rules.find((r) => r.id === rid);
    if (rule && !ruleMatchesURL(rule, url)) {
      return { ok: false, message: formatRuleDomainMismatchMessage(url, rule.domain) };
    }
    return { ok: true, id: rid };
  }
  const matched = rules.filter((r) => ruleMatchesURL(r, url));
  if (matched.length === 0) {
    try {
      const host = new URL(url).hostname.toLowerCase();
      const suggested = suggestRuleDomainForHost(host);
      return {
        ok: false,
        message: `未找到匹配的启用规则。链接主机名为 ${host}，请创建域名为 ${suggested} 的规则，或手动选择规则。`,
      };
    } catch {
      return { ok: false, message: '没有找到可用采集规则，请先创建规则或手动选择' };
    }
  }
  const best = [...matched].sort((a, b) => (b.priority ?? 0) - (a.priority ?? 0))[0];
  return { ok: true, id: best.id };
}

function PlatformConflictAlert({
  hint,
  onDismissPlanned,
  onUseDedicated,
}: {
  hint: CustomCollectPlatformHint;
  onDismissPlanned: () => void;
  onUseDedicated: (source: string) => void;
}) {
  const isBlocked = hint.kind === 'blocked';
  return (
    <Alert
      type={isBlocked ? 'warning' : 'info'}
      showIcon
      style={{ marginBottom: 16 }}
      message={isBlocked ? '请使用专用采集器' : hint.platformLabel}
      description={hint.message}
      action={
        isBlocked ? (
          <Button type="primary" size="small" onClick={() => onUseDedicated(hint.source)}>
            {hint.actionLabel}
          </Button>
        ) : (
          <Button size="small" onClick={onDismissPlanned}>
            {hint.actionLabel}
          </Button>
        )
      }
    />
  );
}

function isLoginLikelyHost(url: string): boolean {
  try {
    const host = new URL(url.trim()).hostname.toLowerCase();
    return host.includes('jd.com') || host.includes('tmall.com') || host.includes('taobao.com');
  } catch {
    return false;
  }
}

export function CustomCollectModal({ open, onClose }: Props) {
  const [rules, setRules] = useState<CollectRuleRow[]>([]);
  const [loadingRules, setLoadingRules] = useState(false);
  const [testing, setTesting] = useState(false);
  const [testResult, setTestResult] = useState<CollectRuleTestResult | null>(null);
  const [formUrl, setFormUrl] = useState('');
  const [formRuleId, setFormRuleId] = useState<string | undefined>();
  const [profileId, setProfileId] = useState<string | undefined>();
  const [useBrowserProfile, setUseBrowserProfile] = useState(false);
  const [plannedDismissed, setPlannedDismissed] = useState(false);
  const [aiModalOpen, setAiModalOpen] = useState(false);

  const matchedRules = useMemo(() => {
    const url = formUrl.trim();
    if (!url) return [];
    return rules.filter((r) => ruleMatchesURL(r, url));
  }, [formUrl, rules]);

  const noRuleForUrl = formUrl.trim().length > 0 && matchedRules.length === 0 && rules.length > 0;

  const platformHint = useMemo(() => {
    const url = formUrl.trim();
    if (!url) return null;
    return detectCustomCollectPlatform(url);
  }, [formUrl]);

  const submitBlocked = platformHint?.kind === 'blocked';

  const ruleOptions = useMemo(
    () =>
      rules.map((r) => ({
        label: `${r.name}（${r.domain} · p${r.priority}）`,
        value: r.id,
      })),
    [rules],
  );

  useEffect(() => {
    if (!open) return;
    setTestResult(null);
    setFormUrl('');
    setFormRuleId(undefined);
    setProfileId(undefined);
    setUseBrowserProfile(false);
    setPlannedDismissed(false);
    setLoadingRules(true);
    void queryCollectRules({ page: 1, pageSize: 500, status: 'enabled' })
      .then((res) => setRules(res.list ?? []))
      .catch(() => setRules([]))
      .finally(() => setLoadingRules(false));
  }, [open]);

  useEffect(() => {
    setPlannedDismissed(false);
    setTestResult(null);
  }, [formUrl]);

  const goToDedicatedCollector = (source: string) => {
    onClose();
    history.push(`/collect/tasks?source=${encodeURIComponent(source)}`);
  };

  const runAccessTest = async () => {
    if (submitBlocked) return;
    const url = formUrl.trim();
    if (!url) {
      message.warning('请先填写商品链接');
      return;
    }
    if (rules.length === 0) {
      message.warning(NO_COLLECT_RULE_MESSAGE);
      return;
    }
    const picked = resolveRuleId(rules, url, formRuleId);
    if (!picked.ok) {
      message.error(picked.message);
      return;
    }
    setTesting(true);
    setTestResult(null);
    try {
      const res = await testCollectRule(picked.id, {
        url,
        profileId: useBrowserProfile ? profileId : undefined,
        useBrowserProfile: useBrowserProfile && Boolean(profileId),
      });
      setTestResult(res);
      if (res.accessStatus === 'public' && !res.missingFields?.length) {
        message.success('访问正常，核心字段已提取');
      } else if (res.accessStatus === 'login_required' || res.accessStatus === 'verify_required') {
        message.warning(res.suggestion || '页面访问受限，请查看测试结果');
      } else {
        message.info(res.suggestion || '测试完成，请查看字段提取情况');
      }
    } catch (e) {
      message.error(mapCollectErrorMessage(e));
    } finally {
      setTesting(false);
    }
  };

  const showPlannedHint =
    platformHint?.kind === 'planned' && !plannedDismissed;

  return (
    <>
    <ModalForm<{ url: string; ruleId?: string }>
      title="自定义链接采集"
      open={open}
      modalProps={{
        destroyOnHidden: true,
        width: 640,
        onCancel: onClose,
      }}
      onOpenChange={(v) => {
        if (!v) onClose();
      }}
      submitter={{
        searchConfig: { submitText: '提交采集任务' },
        submitButtonProps: { disabled: submitBlocked },
        render: (_, dom) => (
          <Space size="middle" wrap className="tm-action-space" style={{ justifyContent: 'flex-end', width: '100%' }}>
            <Button type="link" onClick={() => history.push('/collect/rules')}>
              管理采集规则
            </Button>
            {dom}
          </Space>
        ),
      }}
      onFinish={async (vals) => {
        if (submitBlocked) return false;
        const url = vals.url?.trim();
        if (!url) {
          message.warning('请填写商品链接');
          return false;
        }
        if (rules.length === 0) {
          message.warning(NO_COLLECT_RULE_MESSAGE);
          return false;
        }
        const picked = resolveRuleId(rules, url, vals.ruleId);
        if (!picked.ok) {
          message.error(picked.message);
          return false;
        }
        if (isLoginLikelyHost(url) && !(useBrowserProfile && profileId)) {
          message.warning('该站点通常需要登录态才能看到商品信息，请先启用「已登录采集浏览器」并选择 Profile');
          return false;
        }
        if (testResult?.missingFields?.includes('title') && !(useBrowserProfile && profileId)) {
          message.warning('测试未识别商品标题，建议先启用登录态并重新测试后再提交');
          return false;
        }
        try {
          await createCollectTask({
            source: 'custom',
            url,
            ruleId: vals.ruleId?.trim() || picked.id,
            profileId: useBrowserProfile ? profileId : undefined,
            useBrowserProfile: useBrowserProfile && Boolean(profileId),
          });
          message.success(COLLECT_SUCCESS_SHOP_HINT, 6);
          onClose();
          history.push('/collect/tasks?source=custom');
          return true;
        } catch (e) {
          message.error(mapCollectErrorMessage(e));
          return false;
        }
      }}
    >
      <Alert
        type="info"
        showIcon
        style={{ marginBottom: 16 }}
        message="使用说明"
        description={
          <ul style={{ margin: '8px 0 0', paddingLeft: 20 }}>
            {CUSTOM_COLLECT_USAGE_LINES.map((line) => (
              <li key={line}>{line}</li>
            ))}
          </ul>
        }
      />
      {platformHint && (platformHint.kind === 'blocked' || showPlannedHint) ? (
        <PlatformConflictAlert
          hint={platformHint}
          onDismissPlanned={() => setPlannedDismissed(true)}
          onUseDedicated={goToDedicatedCollector}
        />
      ) : null}
      {loadingRules ? null : rules.length === 0 ? (
        <Alert
          type="warning"
          showIcon
          style={{ marginBottom: 16 }}
          message={NO_COLLECT_RULE_MESSAGE}
          description={
            <Space direction="vertical" size="small">
              <span>可使用 AI 根据商品页面自动生成规则，或手动创建。</span>
              <Space wrap>
                <Button type="primary" size="small" onClick={() => setAiModalOpen(true)}>
                  AI 帮我生成规则
                </Button>
                <Button size="small" onClick={() => history.push('/collect/rules')}>
                  去采集规则页面手动创建
                </Button>
              </Space>
            </Space>
          }
        />
      ) : null}
      {noRuleForUrl && !submitBlocked ? (
        <Alert
          type="info"
          showIcon
          style={{ marginBottom: 16 }}
          message="未找到匹配规则"
          description={
            <Space direction="vertical" size="small">
              <span>是否使用 AI 根据该页面生成规则？</span>
              <Button type="primary" size="small" onClick={() => setAiModalOpen(true)}>
                AI 帮我生成规则
              </Button>
            </Space>
          }
        />
      ) : null}
      <ProFormText
        name="url"
        label="商品详情链接"
        placeholder="https://example.com/product/..."
        rules={[{ required: true, message: '必填' }]}
        fieldProps={{
          onChange: (e) => {
            setFormUrl(e.target.value);
            setTestResult(null);
          },
        }}
      />
      <ProFormSelect
        name="ruleId"
        label="采集规则"
        allowClear
        placeholder="留空则按域名与优先级自动匹配"
        options={ruleOptions}
        fieldProps={{
          disabled: submitBlocked,
          onChange: (v) => {
            setFormRuleId(typeof v === 'string' ? v : undefined);
            setTestResult(null);
          },
        }}
      />
      <Space style={{ marginBottom: testResult ? 8 : 16 }}>
        <Button loading={testing} disabled={submitBlocked} onClick={() => void runAccessTest()}>
          测试采集效果
        </Button>
        <span style={{ color: 'rgba(0,0,0,0.45)', fontSize: 12 }}>
          不提交采集任务，仅检测页面能否打开、商品信息能否识别
        </span>
      </Space>
      {testing ? (
        <div style={{ textAlign: 'center', padding: 16 }}>
          <Spin tip="正在打开页面并检测…" />
        </div>
      ) : null}
      {formUrl.trim() && isLoginLikelyHost(formUrl) && !(useBrowserProfile && profileId) ? (
        <Alert
          type="warning"
          showIcon
          style={{ marginBottom: 16 }}
          message="该站点建议使用已登录采集浏览器"
          description="京东 / 淘宝 / 天猫商品页在未登录时往往看不到标题、价格和主图，采集会失败。"
        />
      ) : null}
      <BrowserProfileLoginPanel
        tone="optional"
        url={formUrl}
        profileId={profileId}
        useBrowserProfile={useBrowserProfile}
        onProfileIdChange={setProfileId}
        onUseProfileChange={setUseBrowserProfile}
      />
      {testResult ? <RuleTestResultPanel result={testResult} /> : null}
      {testResult?.accessStatus === 'login_required' ? (
        <Alert
          type="error"
          showIcon
          style={{ marginTop: 12 }}
          message="页面需要登录"
          description="请在上方面板选择 Profile 并完成登录，然后重新测试后再提交采集任务。"
        />
      ) : null}
      {useBrowserProfile && profileId && testResult?.accessStatus !== 'login_required' ? (
        <Alert
          type="info"
          showIcon
          style={{ marginTop: 12 }}
          message="已启用登录状态"
          description="提交采集任务时将使用所选登录状态（采集浏览器中的登录信息）。"
        />
      ) : null}
    </ModalForm>
    <AIGenerateRuleModal
      open={aiModalOpen}
      initialUrl={formUrl.trim() || undefined}
      onClose={() => setAiModalOpen(false)}
      onSaved={() => {
        setLoadingRules(true);
        void queryCollectRules({ page: 1, pageSize: 500, status: 'enabled' })
          .then((res) => setRules(res.list ?? []))
          .catch(() => setRules([]))
          .finally(() => setLoadingRules(false));
      }}
    />
    </>
  );
}
