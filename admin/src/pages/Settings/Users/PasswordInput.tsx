import { CopyOutlined, ReloadOutlined } from '@ant-design/icons';
import { Button, Input, Space, message } from 'antd';
import { useRef, useState } from 'react';
import { copyPassword, generatePassword } from '@/utils/password';

type Props = {
  id?: string;
  value?: string;
  onChange?: (value: string) => void;
  disabled?: boolean;
  readOnly?: boolean;
};

export default function PasswordInput({ id, value = '', onChange, disabled, readOnly }: Props) {
  const copyingRef = useRef(false);
  const [copying, setCopying] = useState(false);
  const copy = async () => {
    if (disabled || !value || copyingRef.current) return;
    copyingRef.current = true;
    setCopying(true);
    try {
      await copyPassword(value);
      message.success('密码已复制，请妥善保管');
    } catch {
      message.error('复制失败，请显示密码后手动复制');
    } finally {
      copyingRef.current = false;
      setCopying(false);
    }
  };

  return <div>
    <Input.Password id={id} value={value} disabled={disabled} readOnly={readOnly}
      aria-label={readOnly ? '一次性临时密码' : undefined} autoComplete="new-password"
      onChange={(event) => onChange?.(event.target.value)} />
    <Space wrap size={8} style={{ marginTop: 8 }}>
      {!readOnly && <Button size="small" aria-label="随机生成密码" icon={<ReloadOutlined aria-hidden="true" />} disabled={disabled}
        onClick={() => {
          try {
            onChange?.(generatePassword());
            message.success('已生成 20 位随机密码，请复制后妥善保存');
          } catch {
            message.error('当前浏览器无法安全生成密码，请手动输入');
          }
        }}>随机生成</Button>}
      <Button size="small" aria-label="复制密码" icon={<CopyOutlined aria-hidden="true" />} disabled={disabled || !value || copying}
        loading={copying} onClick={() => void copy()}>复制密码</Button>
    </Space>
  </div>;
}
