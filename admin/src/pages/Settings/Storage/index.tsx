import { Link } from '@umijs/renderer-react';
import {
  CloudOutlined,
  CloudServerOutlined,
  CloudUploadOutlined,
  DatabaseOutlined,
  FolderOpenOutlined,
  GlobalOutlined,
  HddOutlined,
  ReloadOutlined,
  SaveOutlined,
  ThunderboltOutlined,
} from '@ant-design/icons';
import { ProCard } from '@ant-design/pro-components';
import { TmPageContainer } from '@/components/ui';
import {
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
import { PAGE_COPY } from '@/constants/copywriting';
import { storageConnectionSectionTitle } from '@/constants/storageSettings';
import { deleteFile, uploadFile, type UploadedFileInfo } from '@/services/files';
import { fetchSettingsList, saveSettingsItems, testStorageConnection, type SettingPutItem } from '@/services/settings';
import { testStoragePublicAccess } from '@/services/douyinProduction';
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
    desc: '标准 AWS S3 区域 + 存储桶（可留空接口地址以使用默认分区）',
    Icon: CloudServerOutlined,
  },
  {
    value: 'cos',
    title: '腾讯云 COS',
    desc: '原生 COS SDK（上传/下载/删除/获取链接），密钥仅存库 AES-GCM 加密',
    Icon: CloudOutlined,
  },
  {
    value: 'oss',
    title: '阿里云 OSS',
    desc: '原生 OSS SDK（上传/下载/删除/获取链接），密钥仅存库 AES-GCM 加密',
    Icon: GlobalOutlined,
  },
  {
    value: 'r2',
    title: 'Cloudflare R2',
    desc: 'S3 兼容接口；区域常为 auto',
    Icon: ThunderboltOutlined,
  },
  {
    value: 'minio',
    title: 'MinIO',
    desc: '私有化对象存储（建议路径式访问）',
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
  const [publicTesting, setPublicTesting] = useState(false);
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
    <TmPageContainer title={PAGE_COPY.storageSettings.title} subTitle={PAGE_COPY.storageSettings.description}>
      <div className="tm-storage-settings">
        <ProCard variant="outlined" className="tm-system-settings__hero">
          <div className="tm-system-settings__hero-inner">
            <div className="tm-system-settings__hero-icon">
              <HddOutlined />
            </div>
            <div className="tm-system-settings__hero-body">
              <Typography.Title level={5} className="tm-system-settings__hero-title">
                自备云存储与访问域名
              </Typography.Title>
              <Typography.Paragraph type="secondary" className="tm-system-settings__hero-desc">
                请在 AWS、Cloudflare R2、MinIO、腾讯云 COS、阿里云 OSS 等开通存储桶与密钥；密钥仅在后端加密保存，浏览器不直连对象存储。上传走服务端接口，可在本页或「文件管理」中测试。总览见{' '}
                <Link to="/settings/integrations">第三方集成总览</Link>。
              </Typography.Paragraph>
            </div>
          </div>
        </ProCard>

        <Form
          form={form}
          layout="vertical"
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
          <ProCard
            variant="outlined"
            title="存储方式"
            className="tm-system-settings__panel"
            extra={
              <Button type="link" icon={<ReloadOutlined />} onClick={() => void load()} disabled={loading}>
                重新加载
              </Button>
            }
          >
            <Form.Item label="选择存储后端" name="kind" rules={[{ required: true, message: '请选择存储方式' }]}>
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
          </ProCard>

          <ProCard variant="outlined" title={storageConnectionSectionTitle(kind)} className="tm-system-settings__panel">
            {kind === 'local' || !kind ? (
              <Row gutter={[24, 0]}>
                <Col xs={24} md={12}>
                  <Form.Item
                    label="公开访问前缀"
                    name="public_base"
                    extra="可填 /static 或完整 URL 前缀，与后端静态资源路由一致"
                  >
                    <Input placeholder="/static 或 http://127.0.0.1:8080/static" />
                  </Form.Item>
                </Col>
                <Col xs={24} md={12}>
                  <Form.Item
                    label="本地保存目录"
                    name="local_root"
                    rules={[{ required: true, message: '请输入本地保存目录' }]}
                    extra="相对项目根目录，如 data/uploads"
                  >
                    <Input placeholder="data/uploads" />
                  </Form.Item>
                </Col>
              </Row>
            ) : null}

            {showS3Form ? (
              <>
                <Row gutter={[24, 0]}>
                  <Col xs={24} md={14}>
                    <Form.Item
                      label="接口地址"
                      name="s3_endpoint"
                      rules={
                        kind === 'r2' || kind === 'minio'
                          ? [{ required: true, message: '请填写接口地址（含 https:// 或 http://）' }]
                          : []
                      }
                      extra={
                        kind === 's3'
                          ? 'AWS 官方分区可留空；R2 / MinIO 需填完整地址'
                          : 'R2: https://<account_id>.r2.cloudflarestorage.com；MinIO: http://127.0.0.1:9000'
                      }
                    >
                      <Input placeholder="https://s3.amazonaws.com 或 MinIO / R2 地址" />
                    </Form.Item>
                  </Col>
                  <Col xs={24} md={10}>
                    <Form.Item label="区域 Region" name="s3_region">
                      <Input placeholder={kind === 'r2' ? 'auto' : kind === 'minio' ? 'us-east-1' : 'ap-southeast-1'} />
                    </Form.Item>
                  </Col>
                </Row>
                <Row gutter={[24, 0]}>
                  <Col xs={24} md={12}>
                    <Form.Item label="存储桶" name="s3_bucket" rules={[{ required: true, message: '请填写存储桶名称' }]}>
                      <Input placeholder="my-bucket" />
                    </Form.Item>
                  </Col>
                  <Col xs={24} md={12}>
                    <Form.Item label="访问密钥 ID" name="s3_access_key_id" rules={[{ required: true }]}>
                      <Input.Password autoComplete="new-password" placeholder="AKIA… 或 R2 令牌" />
                    </Form.Item>
                  </Col>
                </Row>
                <Form.Item label="访问密钥" name="s3_secret_access_key" rules={[{ required: true }]}>
                  <Input.Password autoComplete="new-password" placeholder="保存后脱敏；留空则不修改" />
                </Form.Item>
                <Row gutter={[24, 0]}>
                  <Col xs={24} md={12}>
                    <Form.Item label="路径式访问" name="s3_force_path_style">
                      <Radio.Group>
                        <Radio value="false">虚拟主机（AWS 默认）</Radio>
                        <Radio value="true">路径式（MinIO 常用）</Radio>
                      </Radio.Group>
                    </Form.Item>
                  </Col>
                  <Col xs={24} md={12}>
                    <Form.Item label="传输加密" name="s3_use_ssl">
                      <Radio.Group>
                        <Radio value="true">优先 HTTPS</Radio>
                        <Radio value="false">允许 HTTP（内网 MinIO）</Radio>
                      </Radio.Group>
                    </Form.Item>
                  </Col>
                </Row>
                <Form.Item label="预签名下载 URL" name="s3_presign_enabled">
                  <Radio.Group>
                    <Radio value="false">关闭（推荐：配置对外 URL 前缀）</Radio>
                    <Radio value="true">启用（返回短期签名链接）</Radio>
                  </Radio.Group>
                </Form.Item>
                {presignOn === 'true' ? (
                  <Form.Item label="预签名有效期（秒）" name="s3_presign_expire_seconds">
                    <InputNumber min={60} max={604800} style={{ width: '100%' }} placeholder="3600" />
                  </Form.Item>
                ) : null}
                <Form.Item
                  label="对外访问 URL 前缀"
                  name="s3_public_base"
                  dependencies={['s3_presign_enabled']}
                  rules={[
                    {
                      validator: async (_rule, v) => {
                        if (presignOn === 'true') {
                          return;
                        }
                        if (!String(v || '').trim()) {
                          throw new Error('请填写对外 URL 前缀，或启用预签名下载');
                        }
                      },
                    },
                  ]}
                  extra="CDN / 自定义域名；外链为 prefix + / + objectKey"
                >
                  <Input placeholder="https://cdn.example.com/my-bucket" />
                </Form.Item>
              </>
            ) : null}
            {showCosForm ? (
              <>
                <Row gutter={[24, 0]}>
                  <Col xs={24} md={12}>
                    <Form.Item
                      label="存储桶"
                      name="cos_bucket"
                      rules={[{ required: true, message: '请填写 COS 存储桶' }]}
                      extra="腾讯云名称形如 example-1250000000（含 AppID 后缀）"
                    >
                      <Input placeholder="example-1250000000" />
                    </Form.Item>
                  </Col>
                  <Col xs={24} md={12}>
                    <Form.Item
                      label="区域"
                      name="cos_region"
                      rules={[{ required: true, message: '请填写区域' }]}
                      extra="例如 ap-guangzhou"
                    >
                      <Input placeholder="ap-guangzhou" />
                    </Form.Item>
                  </Col>
                </Row>
                <Row gutter={[24, 0]}>
                  <Col xs={24} md={12}>
                    <Form.Item label="密钥 ID" name="cos_secret_id" rules={[{ required: true }]}>
                      <Input.Password autoComplete="new-password" placeholder="保存后脱敏；留空则不修改" />
                    </Form.Item>
                  </Col>
                  <Col xs={24} md={12}>
                    <Form.Item label="密钥" name="cos_secret_key" rules={[{ required: true }]}>
                      <Input.Password autoComplete="new-password" />
                    </Form.Item>
                  </Col>
                </Row>
                <Row gutter={[24, 0]}>
                  <Col xs={24} md={12}>
                    <Form.Item label="AppID（可选）" name="cos_app_id" extra="桶名为裸名称时需填写，用于拼接域名">
                      <Input placeholder="1250000000" />
                    </Form.Item>
                  </Col>
                  <Col xs={24} md={12}>
                    <Form.Item label="自定义接口地址（可选）" name="cos_endpoint" extra="加速域等特殊入口；留空使用标准域名">
                      <Input placeholder="https://…" />
                    </Form.Item>
                  </Col>
                </Row>
                <Form.Item label="对外 URL 前缀（可选）" name="cos_public_base" extra="CDN 域名；留空则使用桶 REST 域名">
                  <Input placeholder="https://your-cdn.example.com" />
                </Form.Item>
                <Form.Item label="使用 HTTPS" name="cos_use_https">
                  <Radio.Group>
                    <Radio value="true">HTTPS</Radio>
                    <Radio value="false">HTTP（内网）</Radio>
                  </Radio.Group>
                </Form.Item>
              </>
            ) : null}

            {showOssForm ? (
              <>
                <Form.Item
                  label="接口地址"
                  name="oss_endpoint"
                  rules={[{ required: true, message: '请填写 OSS 接口地址' }]}
                  extra="例如 https://oss-cn-guangzhou.aliyuncs.com"
                >
                  <Input placeholder="https://oss-cn-guangzhou.aliyuncs.com" />
                </Form.Item>
                <Row gutter={[24, 0]}>
                  <Col xs={24} md={12}>
                    <Form.Item label="存储桶" name="oss_bucket" rules={[{ required: true, message: '请填写存储桶名称' }]}>
                      <Input placeholder="trademind-assets" />
                    </Form.Item>
                  </Col>
                  <Col xs={24} md={12}>
                    <Form.Item label="访问密钥 ID" name="oss_access_key_id" rules={[{ required: true }]}>
                      <Input.Password autoComplete="new-password" />
                    </Form.Item>
                  </Col>
                </Row>
                <Form.Item label="访问密钥" name="oss_access_key_secret" rules={[{ required: true }]}>
                  <Input.Password autoComplete="new-password" placeholder="保存后脱敏；留空则不修改" />
                </Form.Item>
                <Form.Item label="对外 URL 前缀（可选）" name="oss_public_base" extra="CDN / 自定义域名；留空使用虚拟托管域名">
                  <Input placeholder="https://cdn.example.com" />
                </Form.Item>
                <Form.Item label="使用 HTTPS" name="oss_use_https">
                  <Radio.Group>
                    <Radio value="true">HTTPS</Radio>
                    <Radio value="false">HTTP（内网）</Radio>
                  </Radio.Group>
                </Form.Item>
              </>
            ) : null}
          </ProCard>

          <ProCard variant="outlined" className="tm-system-settings__footer">
            <Space wrap className="tm-action-space">
              <Button type="primary" htmlType="submit" loading={loading} icon={<SaveOutlined />}>
                保存配置
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
              <Button
                loading={publicTesting}
                onClick={async () => {
                  setPublicTesting(true);
                  try {
                    const res = await testStoragePublicAccess();
                    if (res.ok) {
                      message.success(res.message || '图片存储可以被外部平台正常访问');
                    } else {
                      message.error(res.message || '图片地址无法被外部平台访问');
                    }
                  } catch (e: unknown) {
                    message.error((e as Error)?.message || '公网访问检测失败');
                  } finally {
                    setPublicTesting(false);
                  }
                }}
              >
                测试公网访问
              </Button>
            </Space>
          </ProCard>
        </Form>

        <ProCard variant="outlined" title="上传测试" className="tm-system-settings__panel">
          <Typography.Paragraph type="secondary" style={{ marginBottom: 12 }}>
            保存配置后，可选择图片验证上传与访问是否正常。
          </Typography.Paragraph>
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
                  message.success('已删除文件记录与存储对象');
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
              <Button loading={uploading} icon={<CloudUploadOutlined />}>
                选择图片并上传
              </Button>
            </Upload>
            {uploadTestFile ? (
              <Space direction="vertical" size="small">
                <Typography.Text type="secondary">文件 ID</Typography.Text>
                <Typography.Paragraph copyable style={{ marginBottom: 0, maxWidth: 480 }}>
                  {uploadTestFile.id}
                </Typography.Paragraph>
                <Typography.Text type="secondary">访问 URL</Typography.Text>
                <Typography.Paragraph copyable style={{ marginBottom: 0, maxWidth: 480 }}>
                  {uploadTestFile.url}
                </Typography.Paragraph>
                <Image src={uploadTestFile.url} alt="upload" width={200} style={{ objectFit: 'contain' }} />
              </Space>
            ) : null}
          </Space>
        </ProCard>
      </div>
    </TmPageContainer>
  );
}
