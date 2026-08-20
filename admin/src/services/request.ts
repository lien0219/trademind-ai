import { request } from '@umijs/max';

/** 后端统一返回结构（与 Gin Envelope 对齐） */
export type ApiResponse<T> = {
  code: number;
  message: string;
  data: T;
  traceId?: string;
};

export type RequestOptions = {
  headers?: Record<string, string>;
};

function withOptions(options?: RequestOptions) {
  return options?.headers ? { headers: options.headers } : {};
}

export class ApiRequestError extends Error {
  code: number;
  traceId?: string;
  data: unknown;

  constructor(res: ApiResponse<unknown>) {
    super(res.message || 'request_failed');
    this.name = 'ApiRequestError';
    this.code = res.code;
    this.traceId = res.traceId;
    this.data = res.data;
  }
}

function responseEnvelopeFromError(error: unknown): ApiResponse<unknown> | null {
  if (!error || typeof error !== 'object') return null;
  const response = (error as { response?: unknown }).response;
  if (!response || typeof response !== 'object') return null;
  const data = (response as { data?: unknown }).data;
  if (!data || typeof data !== 'object') return null;
  const candidate = data as Partial<ApiResponse<unknown>>;
  if (typeof candidate.code !== 'number' || typeof candidate.message !== 'string' || !('data' in candidate)) {
    return null;
  }
  return candidate as ApiResponse<unknown>;
}

function rethrowRequestError(error: unknown): never {
  const responseEnvelope = responseEnvelopeFromError(error);
  if (responseEnvelope) {
    throw new ApiRequestError(responseEnvelope);
  }
  throw error;
}

async function requestEnvelope<T>(path: string, options: Parameters<typeof request>[1]) {
  try {
    return await request<ApiResponse<T>>(path, options);
  } catch (error) {
    rethrowRequestError(error);
  }
}

function unwrap<T>(res: ApiResponse<T>): T {
  if (res.code !== 0) {
    throw new ApiRequestError(res as ApiResponse<unknown>);
  }
  return res.data;
}

/** 通用 GET（后续各模块拆分到独立 service 文件） */
export async function getJSON<T>(path: string): Promise<T> {
  const res = await requestEnvelope<T>(path, { method: 'GET' });
  return unwrap(res);
}

/** 通用 PUT */
export async function putJSON<T, B extends object>(path: string, body: B, options?: RequestOptions): Promise<T> {
  const res = await requestEnvelope<T>(path, {
    method: 'PUT',
    data: body,
    ...withOptions(options),
  });
  return unwrap(res);
}

/** 通用 PATCH */
export async function patchJSON<T, B extends object>(path: string, body: B, options?: RequestOptions): Promise<T> {
  const res = await requestEnvelope<T>(path, {
    method: 'PATCH',
    data: body,
    ...withOptions(options),
  });
  return unwrap(res);
}

/** 通用 POST */
export async function postJSON<T>(path: string, body?: object, options?: RequestOptions): Promise<T> {
  const res = await requestEnvelope<T>(path, {
    method: 'POST',
    data: body,
    ...withOptions(options),
  });
  return unwrap(res);
}

/** GET with query params */
export async function getWithParams<T>(path: string, params?: Record<string, string | number | undefined>): Promise<T> {
  const res = await requestEnvelope<T>(path, {
    method: 'GET',
    params,
  });
  return unwrap(res);
}

/** DELETE */
export async function deleteJSON<T>(path: string): Promise<T> {
  const res = await requestEnvelope<T>(path, { method: 'DELETE' });
  return unwrap(res);
}

/** multipart/form-data（如上传）；由 request 识别 FormData，勿手动设 Content-Type */
export async function postFormData<T>(path: string, data: FormData): Promise<T> {
  const res = await requestEnvelope<T>(path, {
    method: 'POST',
    data,
  });
  return unwrap(res);
}
