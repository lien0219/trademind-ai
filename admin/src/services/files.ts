import { deleteJSON, getWithParams, postFormData } from '@/services/request';

export type UploadedFileInfo = {
  id: string;
  filename: string;
  objectKey: string;
  url: string;
  contentType: string;
  size: number;
};

export type FileRow = {
  id: string;
  filename: string;
  objectKey: string;
  url: string;
  contentType: string;
  size: number;
  storageKind: string;
  createdBy?: string;
  createdAt: string;
};

type ListResponse = {
  list: FileRow[];
  pagination: {
    page: number;
    pageSize: number;
    total: number;
    totalPages: number;
  };
};

export async function uploadFile(file: File | Blob, filename?: string): Promise<UploadedFileInfo> {
  const form = new FormData();
  const name = filename ?? (file instanceof File ? file.name : 'upload');
  form.append('file', file, name);
  return postFormData<UploadedFileInfo>('/api/v1/files/upload', form);
}

export async function fetchFiles(params: {
  page?: number;
  pageSize?: number;
  contentType?: string;
}): Promise<ListResponse> {
  return getWithParams<ListResponse>('/api/v1/files', {
    page: params.page,
    pageSize: params.pageSize,
    contentType: params.contentType || undefined,
  });
}

export async function deleteFile(id: string): Promise<{ ok: boolean }> {
  return deleteJSON<{ ok: boolean }>(`/api/v1/files/${id}`);
}
