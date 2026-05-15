import { request } from '@umijs/max';

/** 后端统一返回结构（与 Gin Envelope 对齐） */
export type ApiResponse<T> = {
  code: number;
  message: string;
  data: T;
  traceId?: string;
};

function unwrap<T>(res: ApiResponse<T>): T {
  if (res.code !== 0) {
    throw new Error(res.message || 'request_failed');
  }
  return res.data;
}

/** 通用 GET（后续各模块拆分到独立 service 文件） */
export async function getJSON<T>(path: string): Promise<T> {
  const res = await request<ApiResponse<T>>(path, { method: 'GET' });
  return unwrap(res);
}

/** 通用 PUT */
export async function putJSON<T, B extends object>(path: string, body: B): Promise<T> {
  const res = await request<ApiResponse<T>>(path, {
    method: 'PUT',
    data: body,
  });
  return unwrap(res);
}

/** 通用 POST */
export async function postJSON<T>(path: string, body?: object): Promise<T> {
  const res = await request<ApiResponse<T>>(path, {
    method: 'POST',
    data: body,
  });
  return unwrap(res);
}

/** GET with query params */
export async function getWithParams<T>(path: string, params?: Record<string, string | number | undefined>): Promise<T> {
  const res = await request<ApiResponse<T>>(path, {
    method: 'GET',
    params,
  });
  return unwrap(res);
}

/** DELETE */
export async function deleteJSON<T>(path: string): Promise<T> {
  const res = await request<ApiResponse<T>>(path, { method: 'DELETE' });
  return unwrap(res);
}

/** multipart/form-data（如上传）；由 request 识别 FormData，勿手动设 Content-Type */
export async function postFormData<T>(path: string, data: FormData): Promise<T> {
  const res = await request<ApiResponse<T>>(path, {
    method: 'POST',
    data,
  });
  return unwrap(res);
}
