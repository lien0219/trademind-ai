/** 鉴权已改为 layout.onPageChange + token；路由不再声明 access，避免 Umi Navigate 与手写跳转冲突。 */
export default function access() {
  return {};
}
