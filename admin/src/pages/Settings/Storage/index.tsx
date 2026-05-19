import { Link } from '@umijs/renderer-react';
import {
  CloudOutlined,
  CloudServerOutlined,
  DatabaseOutlined,
  FolderOpenOutlined,
  GlobalOutlined,
  ThunderboltOutlined,
} from '@ant-design/icons';
import { PageContainer, ProCard } from '@ant-design/pro-components';
import {
  Alert,
  Button,
  Col,
  Form,
  Image,
  Input,
  InputNumber,
  Radio,
  Row,
  Space,
  Typography,
  Upload,
  message,
} from 'antd';
import type { UploadFile } from 'antd';
import type { ComponentType, CSSProperties } from 'react';
import { useCallback, useEffect, useMemo, useState } from 'react';
import { deleteFile, uploadFile, type UploadedFileInfo } from '@/services/files';
import { fetchSettingsList, saveSettingsItems, testStorageConnection, type SettingPutItem } from '@/services/settings';
import { pickGroup } from '@/utils/settingsForm';

const GROUP = 'storage';

const s3CompatKinds = ['s3', 'r2', 'minio'] as const;

function isS3CompatibleKind(kind: unknown): boolean {
  return (s3CompatKinds as readonly string[]).includes(String(kind || '').toLowerCase());
}

function isCOSKind(kind: unknown): boolean {
  return String(kind || '').toLowerCase() === 'cos';
}

function isOSSKind(kind: unknown): boolean {
  return String(kind || '').toLowerCase() === 'oss';
}

type StorageKindMeta = {
  value: string;
  title: string;
  desc: string;
  Icon: ComponentType<{ style?: CSSProperties }>;
};

const STORAGE_KIND_OPTIONS: StorageKindMeta[] = [
  {
    value: 'local',
    title: '本地磁盘',
    desc: '服务端本地目录，开发与小流量部署开箱即用',
    Icon: FolderOpenOutlined,
  },
  {
    value: 's3',
    title: 'Amazon S3',
    desc: '标准 AWS S3 Region + Bucket（可留空 Endpoint 使用默认分区）',
    Icon: CloudServerOutlined,
  },
  {
    value: 'cos',
    title: '腾讯云 COS',
    desc: '原生 COS SDK（Put/Get/Delete/GetURL），密钥仅存库 AES-GCM 加密',
    Icon: CloudOutlined,
  },
  {
    value: 'oss',
    title: '阿里云 OSS',
    desc: '原生 OSS SDK（Put/Get/Delete/GetURL），密钥仅存库 AES-GCM 加密',
    Icon: GlobalOutlined,
  },
  {
    value: 'r2',
    title: 'Cloudflare R2',
    desc: 'S3 兼容 API；Region 常为 auto',
    Icon: ThunderboltOutlined,
  },
  {
    value: 'minio',
    title: 'MinIO',
    desc: '私有化对象存储（建议 Path-style）',
    Icon: DatabaseOutlined,
  },
];

function pushItem(items: SettingPutItem[], item: Omit<SettingPutItem, 'tenantId'> & { tenantId?: number }) {
  const tenantId = item.tenantId ?? 0;
  const { tenantId: _omit, ...rest } = item;
  items.push({ ...rest, tenantId });
}

function buildStoragePutItems(values: Record<string, unknown>): SettingPutItem[] {
  const tenantId = 0;
  const kind = String(values.kind || 'local');
  const items: SettingPutItem[] = [
    {
      tenantId,
      groupKey: GROUP,
      itemKey: 'kind',
      itemValue: kind,
      valueType: 'string',
      isEncrypted: false,
      remark: '',
    },
  ];

  if (kind === 'local') {
    pushItem(items, {
      tenantId,
      groupKey: GROUP,
      itemKey: 'public_base',
      itemValue: String(values.public_base ?? ''),
      valueType: 'string',
      isEncrypted: false,
      remark: '',
    });
    pushItem(items, {
      tenantId,
      groupKey: GROUP,
      itemKey: 'local_root',
      itemValue: String(values.local_root ?? ''),
      valueType: 'string',
      isEncrypted: false,
      remark: '',
    });
    return items;
  }

  if (isS3CompatibleKind(kind)) {
    const pub = String(values.s3_public_base ?? '');
    pushItem(items, {
      tenantId,
      groupKey: GROUP,
      itemKey: 'public_base',
      itemValue: pub,
      valueType: 'string',
      isEncrypted: false,
      remark: 'mirrors s3_public_base for compatibility',
    });
    pushItem(items, {
      tenantId,
      groupKey: GROUP,
      itemKey: 's3_public_base',
      itemValue: pub,
      valueType: 'string',
      isEncrypted: false,
      remark: '',
    });
    pushItem(items, {
      tenantId,
      groupKey: GROUP,
      itemKey: 's3_endpoint',
      itemValue: String(values.s3_endpoint ?? ''),
      valueType: 'string',
      isEncrypted: false,
      remark: '',
    });
    pushItem(items, {
      tenantId,
      groupKey: GROUP,
      itemKey: 's3_region',
      itemValue: String(values.s3_region ?? ''),
      valueType: 'string',
      isEncrypted: false,
      remark: '',
    });
    pushItem(items, {
      tenantId,
      groupKey: GROUP,
      itemKey: 's3_bucket',
      itemValue: String(values.s3_bucket ?? ''),
      valueType: 'string',
      isEncrypted: false,
      remark: '',
    });
    pushItem(items, {
      tenantId,
      groupKey: GROUP,
      itemKey: 's3_access_key_id',
      itemValue: String(values.s3_access_key_id ?? ''),
      valueType: 'string',
      isEncrypted: true,
      remark: '',
    });
    pushItem(items, {
      tenantId,
      groupKey: GROUP,
      itemKey: 's3_secret_access_key',
      itemValue: String(values.s3_secret_access_key ?? ''),
      valueType: 'string',
      isEncrypted: true,
      remark: '',
    });
    pushItem(items, {
      tenantId,
      groupKey: GROUP,
      itemKey: 's3_force_path_style',
      itemValue: String(values.s3_force_path_style ?? 'false'),
      valueType: 'string',
      isEncrypted: false,
      remark: '',
    });
    pushItem(items, {
      tenantId,
      groupKey: GROUP,
      itemKey: 's3_use_ssl',
      itemValue: String(values.s3_use_ssl ?? 'true'),
      valueType: 'string',
      isEncrypted: false,
      remark: '',
    });
    pushItem(items, {
      tenantId,
      groupKey: GROUP,
      itemKey: 's3_presign_enabled',
      itemValue: String(values.s3_presign_enabled ?? 'false'),
      valueType: 'string',
      isEncrypted: false,
      remark: '',
    });
    pushItem(items, {
      tenantId,
      groupKey: GROUP,
      itemKey: 's3_presign_expire_seconds',
      itemValue:
        values.s3_presign_expire_seconds != null && values.s3_presign_expire_seconds !== ''
          ? String(values.s3_presign_expire_seconds)
          : '3600',
      valueType: 'string',
      isEncrypted: false,
      remark: '',
    });
    return items;
  }

  if (isCOSKind(kind)) {
    pushItem(items, {
      tenantId,
      groupKey: GROUP,
      itemKey: 'cos_bucket',
      itemValue: String(values.cos_bucket ?? ''),
      valueType: 'string',
      isEncrypted: false,
      remark: '',
    });
    pushItem(items, {
      tenantId,
      groupKey: GROUP,
      itemKey: 'cos_region',
      itemValue: String(values.cos_region ?? ''),
      valueType: 'string',
      isEncrypted: false,
      remark: '',
    });
    pushItem(items, {
      tenantId,
      groupKey: GROUP,
      itemKey: 'cos_secret_id',
      itemValue: String(values.cos_secret_id ?? ''),
      valueType: 'string',
      isEncrypted: true,
      remark: '',
    });
    pushItem(items, {
      tenantId,
      groupKey: GROUP,
      itemKey: 'cos_secret_key',
      itemValue: String(values.cos_secret_key ?? ''),
      valueType: 'string',
      isEncrypted: true,
      remark: '',
    });
    pushItem(items, {
      tenantId,
      groupKey: GROUP,
      itemKey: 'cos_app_id',
      itemValue: String(values.cos_app_id ?? ''),
      valueType: 'string',
      isEncrypted: false,
      remark: '',
    });
    pushItem(items, {
      tenantId,
      groupKey: GROUP,
      itemKey: 'cos_endpoint',
      itemValue: String(values.cos_endpoint ?? ''),
      valueType: 'string',
      isEncrypted: false,
      remark: '',
    });
    pushItem(items, {
      tenantId,
      groupKey: GROUP,
      itemKey: 'cos_public_base',
      itemValue: String(values.cos_public_base ?? ''),
      valueType: 'string',
      isEncrypted: false,
      remark: '',
    });
    pushItem(items, {
      tenantId,
      groupKey: GROUP,
      itemKey: 'cos_use_https',
      itemValue: String(values.cos_use_https ?? 'true'),
      valueType: 'string',
      isEncrypted: false,
      remark: '',
    });
    return items;
  }

  if (isOSSKind(kind)) {
    pushItem(items, {
      tenantId,
      groupKey: GROUP,
      itemKey: 'oss_endpoint',
      itemValue: String(values.oss_endpoint ?? ''),
      valueType: 'string',
      isEncrypted: false,
      remark: '',
    });
    pushItem(items, {
      tenantId,
      groupKey: GROUP,
      itemKey: 'oss_bucket',
      itemValue: String(values.oss_bucket ?? ''),
      valueType: 'string',
      isEncrypted: false,
      remark: '',
    });
    pushItem(items, {
      tenantId,
      groupKey: GROUP,
      itemKey: 'oss_access_key_id',
      itemValue: String(values.oss_access_key_id ?? ''),
      valueType: 'string',
      isEncrypted: true,
      remark: '',
    });
    pushItem(items, {
      tenantId,
      groupKey: GROUP,
      itemKey: 'oss_access_key_secret',
      itemValue: String(values.oss_access_key_secret ?? ''),
      valueType: 'string',
      isEncrypted: true,
      remark: '',
    });
    pushItem(items, {
      tenantId,
      groupKey: GROUP,
      itemKey: 'oss_public_base',
      itemValue: String(values.oss_public_base ?? ''),
      valueType: 'string',
      isEncrypted: false,
      remark: '',
    });
    pushItem(items, {
      tenantId,
      groupKey: GROUP,
      itemKey: 'oss_use_https',
      itemValue: String(values.oss_use_https ?? 'true'),
      valueType: 'string',
      isEncrypted: false,
      remark: '',
    });
    return items;
  }

  return items;
}

export default function StorageSettingsPage() {
  const [form] = Form.useForm();
  const [loading, setLoading] = useState(false);
  const [testing, setTesting] = useState(false);
  const [uploading, setUploading] = useState(false);
  const [uploadTestFile, setUploadTestFile] = useState<UploadedFileInfo | null>(null);
  const uploadTestList: UploadFile[] = useMemo(() => {
    if (!uploadTestFile) return [];
    return [
      {
        uid: uploadTestFile.id,
        name: uploadTestFile.filename,
        status: 'done',
        url: uploadTestFile.url,
      },
    ];
  }, [uploadTestFile]);
  const kind = Form.useWatch('kind', form);
  const presignOn = Form.useWatch('s3_presign_enabled', form);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const { items } = await fetchSettingsList();
      const g = pickGroup(items, GROUP);
      const effKind = g.kind || 'local';
      const legacyPub = g.s3_public_base || g.public_base || '';
      form.setFieldsValue({
        kind: effKind,
        public_base: g.public_base || '',
        local_root: g.local_root || 'data/uploads',
        s3_endpoint: g.s3_endpoint || g.endpoint || '',
        s3_region: g.s3_region || g.region || '',
        s3_bucket: g.s3_bucket || g.bucket || '',
        s3_access_key_id: g.s3_access_key_id || g.access_key || '',
        s3_secret_access_key: g.s3_secret_access_key || g.secret_key || '',
        s3_public_base: legacyPub,
        s3_force_path_style:
          String(g.s3_force_path_style || '').trim() !== ''
            ? String(g.s3_force_path_style)
            : effKind === 'minio'
              ? 'true'
              : 'false',
        s3_use_ssl: g.s3_use_ssl === 'false' ? 'false' : 'true',
        s3_presign_enabled: g.s3_presign_enabled === 'true' ? 'true' : 'false',
        s3_presign_expire_seconds: g.s3_presign_expire_seconds ? Number(g.s3_presign_expire_seconds) : 3600,
        cos_bucket: g.cos_bucket || '',
        cos_region: g.cos_region || '',
        cos_secret_id: g.cos_secret_id || '',
        cos_secret_key: g.cos_secret_key || '',
        cos_app_id: g.cos_app_id || '',
        cos_endpoint: g.cos_endpoint || '',
        cos_public_base: g.cos_public_base || '',
        cos_use_https: g.cos_use_https === 'false' ? 'false' : 'true',
        oss_endpoint: g.oss_endpoint || '',
        oss_bucket: g.oss_bucket || '',
        oss_access_key_id: g.oss_access_key_id || '',
        oss_access_key_secret: g.oss_access_key_secret || '',
        oss_public_base: g.oss_public_base || '',
        oss_use_https: g.oss_use_https === 'false' ? 'false' : 'true',
      });
    } catch (e: unknown) {
      message.error((e as Error)?.message || '加载失败');
    } finally {
      setLoading(false);
    }
  }, [form]);

  useEffect(() => {
    load();
  }, [load]);

  const showS3Form = isS3CompatibleKind(kind);
  const showCosForm = isCOSKind(kind);
  const showOssForm = isOSSKind(kind);

  useEffect(() => {
    const k = String(kind || '').toLowerCase();
    if (k === 'minio') {
      const cur = form.getFieldValue('s3_force_path_style');
      if (cur === undefined || cur === null || cur === '') {
        form.setFieldValue('s3_force_path_style', 'true');
      }
    }
  }, [kind, form]);

  return (
    <PageContainer title="存储设置">
      <ProCard bordered style={{ marginBottom: 16 }}>
        <Alert
          type="info"
          showIcon
          message="自备云存储与访问域名"
          description={
            <>
              请在 AWS / Cloudflare R2 / MinIO / 腾讯云 COS / 阿里云 OSS 等开通 Bucket 与密钥；密钥仅在后端加密保存，浏览器不直连对象存储，上传请使用本页测试或「文件管理」走{' '}
              <Typography.Text code>/api/v1/files/upload</Typography.Text>。总览见{' '}
              <Link to="/settings/integrations">第三方集成总览</Link>。
            </>
          }
        />
      </ProCard>
      <ProCard
        bordered
        extra={
          <Button type="link" onClick={load} disabled={loading}>
            重新加载
          </Button>
        }
      >
        <Form
          form={form}
          layout="vertical"
          style={{ maxWidth: 920 }}
          onFinish={async (values) => {
            try {
              await saveSettingsItems(buildStoragePutItems(values as Record<string, unknown>));
              message.success('已保存');
              await load();
            } catch (e: unknown) {
              message.error((e as Error)?.message || '保存失败');
            }
          }}
        >
          <Form.Item label="存储方式" name="kind" rules={[{ required: true, message: '请选择存储方式' }]}>
            <Radio.Group className="tm-storage-kind-group">
              <Row gutter={[12, 12]}>
                {STORAGE_KIND_OPTIONS.map(({ value, title, desc, Icon }) => (
                  <Col xs={24} sm={12} lg={8} key={value}>
                    <Radio value={value} className="tm-storage-kind-radio">
                      <div className="tm-storage-kind-card-main">
                        <span className="tm-storage-kind-icon">
                          <Icon />
                        </span>
                        <div className="tm-storage-kind-text">
                          <div className="tm-storage-kind-title-row">
                            <span className="tm-storage-kind-title">{title}</span>
                          </div>
                          <div className="tm-storage-kind-desc">{desc}</div>
                        </div>
                      </div>
                    </Radio>
                  </Col>
                ))}
              </Row>
            </Radio.Group>
          </Form.Item>

          {kind === 'local' || !kind ? (
            <>
              <Form.Item
                label="公开访问前缀 URL"
                name="public_base"
                extra="可填 /static 或完整 URL 前缀，与后端 GET /static 或反代一致"
              >
                <Input placeholder="/static 或 http://127.0.0.1:8080/static" />
              </Form.Item>
              <Form.Item
                label="本地根目录"
                name="local_root"
                rules={[{ required: true }]}
                extra="相对路径以仓库根目录为准（如 data/uploads → 项目根/data/uploads），与从 backend 或根目录启动无关"
              >
                <Input placeholder="data/uploads" />
              </Form.Item>
            </>
          ) : null}

          {showS3Form ? (
            <>
              <Form.Item
                label="S3 Endpoint（s3_endpoint）"
                name="s3_endpoint"
                rules={
                  kind === 'r2' || kind === 'minio'
                    ? [{ required: true, message: '请填写 Endpoint URL（含协议，如 https:// 或 http://）' }]
                    : []
                }
                extra={
                  kind === 's3'
                    ? '使用 AWS 官方分区时可留空；自定义网关 / MinIO / R2 需填完整 Base URL'
                    : '示例 R2: https://<account_id>.r2.cloudflarestorage.com；MinIO: http://127.0.0.1:9000'
                }
              >
                <Input placeholder="https://s3.amazonaws.com 或 MinIO / R2 地址" />
              </Form.Item>
              <Form.Item label="Region（s3_region）" name="s3_region">
                <Input placeholder={kind === 'r2' ? 'auto' : kind === 'minio' ? 'us-east-1' : 'ap-southeast-1'} />
              </Form.Item>
              <Form.Item label="Bucket（s3_bucket）" name="s3_bucket" rules={[{ required: true }]}>
                <Input />
              </Form.Item>
              <Form.Item label="Access Key ID" name="s3_access_key_id" rules={[{ required: true }]}>
                <Input.Password autoComplete="new-password" placeholder="AKIA… 或 R2 token" />
              </Form.Item>
              <Form.Item label="Secret Access Key" name="s3_secret_access_key" rules={[{ required: true }]}>
                <Input.Password autoComplete="new-password" />
              </Form.Item>
              <Form.Item label="Path-style 访问" name="s3_force_path_style">
                <Radio.Group>
                  <Radio value="false">虚拟主机样式（AWS 默认）</Radio>
                  <Radio value="true">Path-style（MinIO 常用）</Radio>
                </Radio.Group>
              </Form.Item>
              <Form.Item label="传输" name="s3_use_ssl">
                <Radio.Group>
                  <Radio value="true">优先 HTTPS（Endpoint 无 scheme 时）</Radio>
                  <Radio value="false">允许 HTTP（如内网 MinIO）</Radio>
                </Radio.Group>
              </Form.Item>
              <Form.Item label="预签名下载 URL" name="s3_presign_enabled">
                <Radio.Group>
                  <Radio value="false">关闭（推荐：配置对外 URL 前缀）</Radio>
                  <Radio value="true">启用（GetURL 返回短期预签名，链接会过期）</Radio>
                </Radio.Group>
              </Form.Item>
              {presignOn === 'true' ? (
                <Form.Item label="预签名有效期（秒）" name="s3_presign_expire_seconds">
                  <InputNumber min={60} max={604800} style={{ width: '100%' }} />
                </Form.Item>
              ) : null}
              <Form.Item
                label="对象公开访问 URL 前缀（s3_public_base）"
                name="s3_public_base"
                dependencies={['s3_presign_enabled']}
                rules={[
                  {
                    validator: async (_rule, v) => {
                      if (presignOn === 'true') {
                        return;
                      }
                      if (!String(v || '').trim()) {
                        throw new Error('请填写对外可访问的 URL 前缀，或启用上方的「预签名下载 URL」');
                      }
                    },
                  },
                ]}
                extra="例如 Cloudflare 自定义域名、CloudFront，或 MinIO 对外网关；外链形式为 prefix + 「/」 + objectKey"
              >
                <Input placeholder="https://cdn.example.com/my-bucket 或 https://pub-xxx.r2.dev" />
              </Form.Item>
            </>
          ) : null}

          {showCosForm ? (
            <>
              <Form.Item
                label="Bucket（cos_bucket）"
                name="cos_bucket"
                rules={[{ required: true, message: '请填写 COS 存储桶标识' }]}
                extra={
                  <>
                    Tencent 侧名称形如 <Typography.Text code>myapp-1250000000</Typography.Text>
                    （含 AppID 后缀）。若填写裸桶名 <Typography.Text code>myapp</Typography.Text>，请在后项填写{' '}
                    <Typography.Text code>cos_app_id</Typography.Text>。
                  </>
                }
              >
                <Input placeholder="example-1250000000" />
              </Form.Item>
              <Form.Item
                label="Region（cos_region）"
                name="cos_region"
                rules={[{ required: true }]}
                extra="例如 ap-guangzhou"
              >
                <Input placeholder="ap-guangzhou" />
              </Form.Item>
              <Form.Item label="SecretId（cos_secret_id）" name="cos_secret_id" rules={[{ required: true }]}>
                <Input.Password autoComplete="new-password" placeholder="仅存库 AES-GCM；列表脱敏后以 **** 表示，不改密钥时请保留" />
              </Form.Item>
              <Form.Item label="SecretKey（cos_secret_key）" name="cos_secret_key" rules={[{ required: true }]}>
                <Input.Password autoComplete="new-password" />
              </Form.Item>
              <Form.Item
                label="AppID（cos_app_id，可选）"
                name="cos_app_id"
                extra='当 cos_bucket 为裸名称（不含 "-"）时用于拼接 Bucket 域名后缀'
              >
                <Input placeholder="1250000000" />
              </Form.Item>
              <Form.Item
                label="自定义 Endpoint（cos_endpoint，可选）"
                name="cos_endpoint"
                extra="自定义加速域 / 万象 CI 等特殊入口时填写；留空则用标准 https://{{bucket}}.cos.{{region}}.myqcloud.com"
              >
                <Input placeholder="https://…" />
              </Form.Item>
              <Form.Item
                label="对外 URL 前缀（cos_public_base，可选）"
                name="cos_public_base"
                extra="CDN 或静态网站托管域名前缀；外链为 prefix/objectKey；留空则使用桶 REST 域名"
              >
                <Input placeholder="https://your-cdn.example.com" />
              </Form.Item>
              <Form.Item label="使用 HTTPS（cos_use_https）" name="cos_use_https">
                <Radio.Group>
                  <Radio value="true">HTTPS</Radio>
                  <Radio value="false">HTTP（内网等）</Radio>
                </Radio.Group>
              </Form.Item>
            </>
          ) : null}

          {showOssForm ? (
            <>
              <Form.Item
                label="OSS Endpoint（oss_endpoint）"
                name="oss_endpoint"
                rules={[{ required: true, message: '请填写 OSS Endpoint URL' }]}
                extra="形如 https://oss-cn-guangzhou.aliyuncs.com（可含 VPC/内网 EndPoint）；与 oss_use_https 一致"
              >
                <Input placeholder="https://oss-cn-guangzhou.aliyuncs.com" />
              </Form.Item>
              <Form.Item label="Bucket（oss_bucket）" name="oss_bucket" rules={[{ required: true }]}>
                <Input placeholder="trademind-assets" />
              </Form.Item>
              <Form.Item label="AccessKey Id（oss_access_key_id）" name="oss_access_key_id" rules={[{ required: true }]}>
                <Input.Password autoComplete="new-password" />
              </Form.Item>
              <Form.Item
                label="AccessKey Secret（oss_access_key_secret）"
                name="oss_access_key_secret"
                rules={[{ required: true }]}
              >
                <Input.Password autoComplete="new-password" />
              </Form.Item>
              <Form.Item
                label="对外 URL 前缀（oss_public_base，可选）"
                name="oss_public_base"
                extra="CDN/自定义域名；留空则用虚拟托管域名：https://bucket + endpoint-host；仅当 Bucket 已对公网可读时可直接外链"
              >
                <Input placeholder="https://cdn.example.com 或 Bucket 绑定域名" />
              </Form.Item>
              <Form.Item label="使用 HTTPS（oss_use_https）" name="oss_use_https">
                <Radio.Group>
                  <Radio value="true">HTTPS</Radio>
                  <Radio value="false">HTTP（内网等）</Radio>
                </Radio.Group>
              </Form.Item>
            </>
          ) : null}

          <Form.Item>
            <Space>
              <Button type="primary" htmlType="submit" loading={loading}>
                保存
              </Button>
              <Button
                loading={testing}
                onClick={async () => {
                  setTesting(true);
                  try {
                    await testStorageConnection();
                    message.success('连接检测通过');
                  } catch (e: unknown) {
                    message.error((e as Error)?.message || '校验失败');
                  } finally {
                    setTesting(false);
                  }
                }}
              >
                测试连接
              </Button>
            </Space>
          </Form.Item>
        </Form>
      </ProCard>
      <ProCard title="上传测试" bordered style={{ marginTop: 16 }}>
        <Space align="start" wrap size="large">
          <Upload
            maxCount={1}
            accept=".jpg,.jpeg,.png,.webp,.gif,image/jpeg,image/png,image/webp,image/gif"
            showUploadList
            fileList={uploadTestList}
            onRemove={async () => {
              if (!uploadTestFile?.id) {
                setUploadTestFile(null);
                return true;
              }
              try {
                await deleteFile(uploadTestFile.id);
                message.success('已删除文件记录与磁盘对象');
                setUploadTestFile(null);
                return true;
              } catch (e: unknown) {
                message.error((e as Error)?.message || '删除失败');
                return false;
              }
            }}
            beforeUpload={(file) => {
              void (async () => {
                setUploading(true);
                setUploadTestFile(null);
                try {
                  const r = await uploadFile(file);
                  setUploadTestFile(r);
                  message.success('上传成功');
                } catch (e: unknown) {
                  message.error((e as Error)?.message || '上传失败');
                } finally {
                  setUploading(false);
                }
              })();
              return false;
            }}
          >
            <Button loading={uploading}>选择图片并上传（走 /api/v1/files/upload）</Button>
          </Upload>
          {uploadTestFile ? (
            <Space direction="vertical" size="small">
              <Typography.Text type="secondary">文件 ID（删除时使用）</Typography.Text>
              <Typography.Paragraph copyable style={{ marginBottom: 0, maxWidth: 480 }}>
                {uploadTestFile.id}
              </Typography.Paragraph>
              <Typography.Text type="secondary">返回 URL</Typography.Text>
              <Typography.Paragraph copyable style={{ marginBottom: 0, maxWidth: 480 }}>
                {uploadTestFile.url}
              </Typography.Paragraph>
              <Image src={uploadTestFile.url} alt="upload" width={200} style={{ objectFit: 'contain' }} />
            </Space>
          ) : null}
        </Space>
      </ProCard>
    </PageContainer>
  );
}
