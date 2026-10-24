import { EyeOutlined, PlusOutlined, ReloadOutlined, SafetyCertificateOutlined, TeamOutlined } from '@ant-design/icons';
import { Button, Modal, Select, Skeleton, Space, Tag, Tooltip } from 'antd';
import { useState } from 'react';
import { EmptyState, OperationToolbar, SectionCard } from '@/components/ui';
import { layoutTokens } from '@/constants/layoutTokens';
import { groupPermissions, permissionLabel, roleLabel } from '@/constants/permissionLabels';
import type { AdminRoleRow } from '@/services/adminUsers';
import './rolePermissions.less';

type Props = {
  roles: AdminRoleRow[];
  loading: boolean;
  ready: boolean;
  hasError: boolean;
  canManage: boolean;
  submitting: boolean;
  onCreate: () => void;
  onRefresh: () => void;
  onEdit: (role: AdminRoleRow) => void;
  onToggle: (role: AdminRoleRow) => void;
  onDelete: (role: AdminRoleRow) => void;
};

const SYSTEM_DESCRIPTIONS: Record<string, string> = {
  admin: '负责系统管理、用户授权与业务配置。',
  operator: '处理日常商品、订单、库存与店铺运营。',
  reviewer: '审核业务操作并查看相关记录。',
  readonly: '查看业务数据与操作记录。',
};

export default function RolePermissionsPanel({ roles, loading, ready, hasError, canManage, submitting, onCreate, onRefresh, onEdit, onToggle, onDelete }: Props) {
  const [compareRole, setCompareRole] = useState<string>();
  const [previewCode, setPreviewCode] = useState<string>();
  const previewRole = roles.find((role) => role.code === previewCode);
  const comparison = roles.find((role) => role.code === compareRole);

  return (
    <div className="tm-role-panel">
      <SectionCard title="角色管理" description="按岗位分配权限，预览授权范围并对比角色差异。" compact>
        <OperationToolbar extra={
          <div className="tm-role-panel__compare">
            <span>对比基准</span>
            <Select aria-label="选择对比角色" allowClear placeholder="选择对比角色" value={comparison?.code}
              disabled={!ready} options={roles.map((role) => ({ label: roleLabel(role.code, role.name), value: role.code }))}
              onChange={setCompareRole} />
          </div>
        }>
          <Button type="primary" icon={<PlusOutlined aria-hidden="true" />} disabled={!canManage || !ready || submitting} onClick={onCreate}>新建角色</Button>
          <Button aria-label="刷新角色" icon={<ReloadOutlined aria-hidden="true" />} loading={loading} onClick={onRefresh}>刷新角色</Button>
        </OperationToolbar>
        <div className="tm-role-panel__summary">共 {roles.length} 个角色<span>已启用 {roles.filter((role) => role.enabled).length} 个</span><span>系统角色由系统维护，自定义角色可按需调整。</span></div>
      </SectionCard>
      {!roles.length && (loading || (!ready && !hasError)) ? <SectionCard><Skeleton active paragraph={{ rows: 4 }} /></SectionCard> : null}
      {!loading && ready && roles.length === 0 && <SectionCard><EmptyState title="暂无角色" description="创建自定义角色，为不同岗位分配合适的权限。" /></SectionCard>}
      <div className="tm-role-panel__grid" aria-label="角色权限管理">
        {roles.map((role) => {
          const label = roleLabel(role.code, role.name);
          const groups = groupPermissions(role.permissions);
          const additions = comparison ? role.permissions.filter((key) => !comparison.permissions.includes(key)) : [];
          const removals = comparison ? comparison.permissions.filter((key) => !role.permissions.includes(key)) : [];
          return (
            <article key={role.code} aria-label={label} className="tm-role-panel__card">
              <SectionCard compact title={<span className="tm-role-panel__role-title"><SafetyCertificateOutlined aria-hidden="true" />{label}</span>}
                headerExtra={<Tag color={role.enabled ? 'success' : 'default'}>{role.enabled ? '启用' : '已停用'}</Tag>}>
                <div className="tm-role-panel__identity"><Tag>{role.system ? '系统角色' : '自定义角色'}</Tag>{role.code === comparison?.code && <Tag color="processing">对比基准</Tag>}</div>
                <p className="tm-role-panel__description">{role.description || (role.system ? SYSTEM_DESCRIPTIONS[role.code] : '') || '按岗位配置所需的页面与操作权限。'}</p>
                <div className="tm-role-panel__counts"><span><TeamOutlined aria-hidden="true" />成员：{role.memberCount} 人</span><span>权限：<strong>{role.permissions.length}</strong> 项</span><span>{groups.length} 个模块</span></div>
                <div className="tm-role-panel__modules" aria-label="已授权模块">
                  {groups.slice(0, 6).map((group) => <Tag key={group.key}>{group.label}</Tag>)}
                  {groups.length > 6 && <span>另 {groups.length - 6} 个模块</span>}
                  {!groups.length && <span>无授权权限</span>}
                </div>
                {comparison && comparison.code !== role.code && (
                  <div className="tm-role-panel__diff">
                    <div className="tm-role-panel__diff-title">相对于「{roleLabel(comparison.code, comparison.name)}」的权限差异</div>
                    <div className="tm-role-panel__diff-tags">
                      {additions.map((key) => <Tag key={key} color="success">多出：{permissionLabel(key)}</Tag>)}
                      {removals.map((key) => <Tag key={key} color="error">缺少：{permissionLabel(key)}</Tag>)}
                      {!additions.length && !removals.length && <Tag>权限一致</Tag>}
                    </div>
                  </div>
                )}
                <div className="tm-role-panel__actions">
                  <Button icon={<EyeOutlined aria-hidden="true" />} onClick={() => setPreviewCode(role.code)}>权限预览</Button>
                  <Space wrap size={8}>
                    <Tooltip title={role.system ? '系统角色由系统维护，无法直接编辑' : undefined}>
                      <span><Button disabled={role.system || !canManage || submitting || !ready} onClick={() => onEdit(role)}>编辑权限</Button></span>
                    </Tooltip>
                    {!role.system && <Button danger={role.enabled} disabled={!canManage || submitting || !ready} onClick={() => onToggle(role)}>{role.enabled ? '停用' : '启用'}</Button>}
                    {!role.system && <Tooltip title={role.memberCount > 0 ? '请先调整关联用户的角色，再删除此角色' : undefined}>
                      <span><Button danger disabled={!canManage || submitting || !ready || role.memberCount > 0} onClick={() => onDelete(role)}>删除角色</Button></span>
                    </Tooltip>}
                  </Space>
                </div>
                {!role.system && role.memberCount > 0 && <p className="tm-role-panel__delete-hint">仍有关联成员，请先调整成员角色后删除。</p>}
              </SectionCard>
            </article>
          );
        })}
      </div>
      <Modal title="角色权限预览" open={!!previewRole} onCancel={() => setPreviewCode(undefined)} width={layoutTokens.modalWidthLg}
        centered footer={<Button onClick={() => setPreviewCode(undefined)}>关闭</Button>} destroyOnHidden>
        {previewRole && <div className="tm-role-preview">
          <div className="tm-role-preview__summary"><strong>{roleLabel(previewRole.code, previewRole.name)}</strong><Tag>{previewRole.permissions.length} 项权限</Tag><Tag>{previewRole.enabled ? '启用' : '已停用'}</Tag></div>
          <p className="tm-role-preview__hint">以下为此角色已配置的权限；实际访问还受账号状态、租户及店铺授权范围约束。</p>
          <div className="tm-role-preview__groups">
            {groupPermissions(previewRole.permissions).map((group) => <section key={group.key} aria-label={group.label} className="tm-role-preview__group">
              <h4>{group.label}<span>{group.permissions.length} 项</span></h4>
              <div className="tm-role-preview__tags">{group.permissions.map((key) => <Tag key={key}>{permissionLabel(key)}</Tag>)}</div>
            </section>)}
            {!previewRole.permissions.length && <EmptyState title="无授权权限" compact />}
          </div>
        </div>}
      </Modal>
    </div>
  );
}
