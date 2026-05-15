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
  Button,
  Col,
  Form,
  Image,
  Input,
  Radio,
  Row,
  Space,
  Tag,
  Typography,
  Upload,
  message,
} from 'antd';
import type { ComponentType, CSSProperties } from 'react';
import { useCallback, useEffect, useState } from 'react';
import { uploadFile } from '@/services/files';
import { fetchSettingsList, saveSettingsItems, testStorageConnection, type SettingPutItem } from '@/services/settings';
import { pickGroup } from '@/utils/settingsForm';

const GROUP = 'storage';

const cloudKinds = ['s3', 'cos', 'oss', 'r2', 'minio'];

type StorageKindMeta = {
  value: string;
  title: string;
  desc: string;
  Icon: ComponentType<{ style?: CSSProperties }>;
  reserved?: boolean;
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
    title: 'S3 兼容',
    desc: 'AWS、自有网关等兼容 S3 协议的 Bucket',
    Icon: CloudServerOutlined,
  },
  {
    value: 'cos',
    title: '腾讯云 COS',
    desc: '对象存储接入占位',
    Icon: CloudOutlined,
    reserved: true,
  },
  {
    value: 'oss',
    title: '阿里云 OSS',
    desc: '对象存储接入占位',
    Icon: GlobalOutlined,
    reserved: true,
  },
  {
    value: 'r2',
    title: 'Cloudflare R2',
    desc: 'S3 兼容接入占位',
    Icon: ThunderboltOutlined,
    reserved: true,
  },
  {
    value: 'minio',
    title: 'MinIO',
    desc: '私有化对象存储接入占位',
    Icon: DatabaseOutlined,
    reserved: true,
  },
];

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
    {
      tenantId,
      groupKey: GROUP,
      itemKey: 'public_base',
      itemValue: String(values.public_base ?? ''),
      valueType: 'string',
      isEncrypted: false,
      remark: '',
    },
  ];
  if (kind === 'local') {
    items.push({
      tenantId,
      groupKey: GROUP,
      itemKey: 'local_root',
      itemValue: String(values.local_root ?? ''),
      valueType: 'string',
      isEncrypted: false,
      remark: '',
    });
  } else if (cloudKinds.includes(kind)) {
    items.push(
      {
        tenantId,
        groupKey: GROUP,
        itemKey: 'endpoint',
        itemValue: String(values.endpoint ?? ''),
        valueType: 'string',
        isEncrypted: false,
        remark: '',
      },
      {
        tenantId,
        groupKey: GROUP,
        itemKey: 'bucket',
        itemValue: String(values.bucket ?? ''),
        valueType: 'string',
        isEncrypted: false,
        remark: '',
      },
      {
        tenantId,
        groupKey: GROUP,
        itemKey: 'region',
        itemValue: String(values.region ?? ''),
        valueType: 'string',
        isEncrypted: false,
        remark: '',
      },
      {
        tenantId,
        groupKey: GROUP,
        itemKey: 'access_key',
        itemValue: String(values.access_key ?? ''),
        valueType: 'string',
        isEncrypted: false,
        remark: '',
      },
      {
        tenantId,
        groupKey: GROUP,
        itemKey: 'secret_key',
        itemValue: String(values.secret_key ?? ''),
        valueType: 'string',
        isEncrypted: true,
        remark: '',
      },
    );
  }
  return items;
}

export default function StorageSettingsPage() {
  const [form] = Form.useForm();
  const [loading, setLoading] = useState(false);
  const [testing, setTesting] = useState(false);
  const [uploading, setUploading] = useState(false);
  const [uploadPreviewUrl, setUploadPreviewUrl] = useState<string | null>(null);
  const kind = Form.useWatch('kind', form);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const { items } = await fetchSettingsList();
      const g = pickGroup(items, GROUP);
      form.setFieldsValue({
        kind: g.kind || 'local',
        public_base: g.public_base || '',
        local_root: g.local_root || 'data/uploads',
        endpoint: g.endpoint || '',
        bucket: g.bucket || '',
        region: g.region || '',
        access_key: g.access_key || '',
        secret_key: g.secret_key || '',
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

  const showCloud = cloudKinds.includes(String(kind || ''));

  return (
    <PageContainer title="存储设置">
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
                {STORAGE_KIND_OPTIONS.map(({ value, title, desc, Icon, reserved }) => (
                  <Col xs={24} sm={12} lg={8} key={value}>
                    <Radio value={value} className="tm-storage-kind-radio">
                      <div className="tm-storage-kind-card-main">
                        <span className="tm-storage-kind-icon">
                          <Icon />
                        </span>
                        <div className="tm-storage-kind-text">
                          <div className="tm-storage-kind-title-row">
                            <span className="tm-storage-kind-title">{title}</span>
                            {reserved ? (
                              <Tag bordered={false} style={{ marginInlineEnd: 0 }}>
                                预留
                              </Tag>
                            ) : null}
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
          <Form.Item
            label="公开访问前缀 URL"
            name="public_base"
            extra="可填 /static 或完整 URL 前缀"
          >
            <Input placeholder="/static 或 http://127.0.0.1:8080/static" />
          </Form.Item>
          {kind === 'local' || !kind ? (
            <Form.Item label="本地根目录" name="local_root" rules={[{ required: true }]}>
              <Input placeholder="data/uploads" />
            </Form.Item>
          ) : null}
          {showCloud ? (
            <>
              <Form.Item label="Endpoint" name="endpoint" rules={[{ required: true, message: '填写 Endpoint' }]}>
                <Input placeholder="https://s3.amazonaws.com 或区域 endpoint" />
              </Form.Item>
              <Form.Item label="Bucket" name="bucket" rules={[{ required: true }]}>
                <Input />
              </Form.Item>
              <Form.Item label="Region" name="region">
                <Input placeholder="可选" />
              </Form.Item>
              <Form.Item label="Access Key" name="access_key" rules={[{ required: true }]}>
                <Input />
              </Form.Item>
              <Form.Item label="Secret Key" name="secret_key" rules={[{ required: true, message: '填写 Secret' }]}>
                <Input.Password autoComplete="new-password" />
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
                    message.success('校验通过');
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
            beforeUpload={(file) => {
              void (async () => {
                setUploading(true);
                setUploadPreviewUrl(null);
                try {
                  const r = await uploadFile(file);
                  setUploadPreviewUrl(r.url);
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
            <Button loading={uploading}>选择图片并上传</Button>
          </Upload>
          {uploadPreviewUrl ? (
            <Space direction="vertical" size="small">
              <Typography.Text type="secondary">返回 URL</Typography.Text>
              <Typography.Paragraph copyable style={{ marginBottom: 0, maxWidth: 480 }}>
                {uploadPreviewUrl}
              </Typography.Paragraph>
              <Image src={uploadPreviewUrl} alt="upload" width={200} style={{ objectFit: 'contain' }} />
            </Space>
          ) : null}
        </Space>
      </ProCard>
    </PageContainer>
  );
}
