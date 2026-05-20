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
import { mapCollectErrorMessage } from '@/constants/collectErrors';
import { BrowserProfileLoginPanel } from '@/pages/Collect/components/BrowserProfileLoginPanel';
import { RuleTestResultPanel } from '@/pages/Collect/components/RuleTestResultPanel';

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
      return { ok: false, message: '未找到匹配的自定义采集规则，请先创建规则或手动选择' };
    }
  }
  const best = [...matched].sort((a, b) => (b.priority ?? 0) - (a.priority ?? 0))[0];
  return { ok: true, id: best.id };
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
    setLoadingRules(true);
    void queryCollectRules({ page: 1, pageSize: 500, status: 'enabled' })
      .then((res) => setRules(res.list ?? []))
      .catch(() => setRules([]))
      .finally(() => setLoadingRules(false));
  }, [open]);

  const runAccessTest = async () => {
    const url = formUrl.trim();
    if (!url) {
      message.warning('请先填写商品链接');
      return;
    }
    if (rules.length === 0) {
      message.error('请先到「采集规则」创建并启用一条自定义采集规则');
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

  return (
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
        render: (_, dom) => (
          <>
            <Button type="link" onClick={() => history.push('/collect/rules')}>
              管理采集规则
            </Button>
            {dom}
          </>
        ),
      }}
      onFinish={async (vals) => {
        const url = vals.url?.trim();
        if (!url) {
          message.warning('请填写商品链接');
          return false;
        }
        if (rules.length === 0) {
          message.error('请先到「采集规则」创建并启用一条自定义采集规则');
          return false;
        }
        const picked = resolveRuleId(rules, url, vals.ruleId);
        if (!picked.ok) {
          message.error(picked.message);
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
          message.success('采集任务已提交');
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
        message="自定义采集通过 CSS 选择器解析页面，不执行用户脚本。批量采集暂未开放。"
        description="1688 域名仍使用专属登录态检测；其他站点使用通用访问状态检测（不自动登录、不破解验证码）。"
      />
      {loadingRules ? null : rules.length === 0 ? (
        <Alert
          type="warning"
          showIcon
          style={{ marginBottom: 16 }}
          message="暂无启用的采集规则"
          description={
            <span>
              请先到
              <a onClick={() => history.push('/collect/rules')}> 采集规则 </a>
              创建规则（填写域名与 selector JSON）后再采集。
            </span>
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
          onChange: (v) => {
            setFormRuleId(typeof v === 'string' ? v : undefined);
            setTestResult(null);
          },
        }}
      />
      <Space style={{ marginBottom: testResult ? 8 : 16 }}>
        <Button loading={testing} onClick={() => void runAccessTest()}>
          测试访问与规则
        </Button>
        <span style={{ color: 'rgba(0,0,0,0.45)', fontSize: 12 }}>
          不创建采集任务，仅检测页面可访问性与选择器提取效果
        </span>
      </Space>
      {testing ? (
        <div style={{ textAlign: 'center', padding: 16 }}>
          <Spin tip="正在打开页面并检测…" />
        </div>
      ) : null}
      {testResult ? <RuleTestResultPanel result={testResult} /> : null}
      {testResult?.accessStatus === 'login_required' ? (
        <BrowserProfileLoginPanel
          url={formUrl}
          profileId={profileId}
          useBrowserProfile={useBrowserProfile}
          onProfileIdChange={setProfileId}
          onUseProfileChange={setUseBrowserProfile}
        />
      ) : null}
      {useBrowserProfile && profileId && testResult?.accessStatus !== 'login_required' ? (
        <Alert
          type="info"
          showIcon
          style={{ marginTop: 12 }}
          message="已启用登录态 Profile"
          description="提交采集任务时将使用所选 Profile 的浏览器登录态。"
        />
      ) : null}
    </ModalForm>
  );
}
