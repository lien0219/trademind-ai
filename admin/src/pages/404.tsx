import { Button, Result } from 'antd';
import { history } from '@umijs/max';

export default function NotFoundPage() {
  return (
    <Result
      status="404"
      title="页面不存在"
      subTitle="请从左侧菜单进入工作台或各业务模块。"
      extra={
        <Button type="primary" onClick={() => history.push('/dashboard')}>
          返回工作台
        </Button>
      }
    />
  );
}
