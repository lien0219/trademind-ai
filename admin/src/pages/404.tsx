import { useLanguage } from '@/i18n/useLanguage';
import { tr } from '@/i18n';
import { Button, Result } from 'antd';
import { history } from '@umijs/max';

export default function NotFoundPage() {
  useLanguage();
  return (
    <Result
      status="404"
      title="404"
      subTitle={tr("页面不存在")}
      extra={
        <Button type="primary" onClick={() => history.push('/dashboard')}>{tr("返回工作台")}</Button>
      }
    />
  );
}
