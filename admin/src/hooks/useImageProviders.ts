import { useCallback, useEffect, useState } from 'react';
import type { ImageProviderCapability } from '@/constants/imageProviders';
import { isProviderSelectable, providersForTask } from '@/constants/imageProviders';
import { fetchImageProviders } from '@/services/imageProviders';

export function useImageProviders() {
  const [caps, setCaps] = useState<ImageProviderCapability[]>([]);
  const [loading, setLoading] = useState(false);

  const reload = useCallback(async () => {
    setLoading(true);
    try {
      const list = await fetchImageProviders();
      setCaps(list);
    } catch {
      setCaps([]);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void reload();
  }, [reload]);

  const optionsForTask = useCallback(
    (taskType: string, includeDefault = true) => {
      const matched = providersForTask(caps, taskType);
      const base = matched.map((c) => ({
        value: c.provider,
        label: c.displayName,
        disabled: !isProviderSelectable(c),
      }));
      if (includeDefault) {
        return [{ value: '', label: '默认（跟随「图片 AI」设置）' }, ...base];
      }
      return base;
    },
    [caps],
  );

  return { caps, loading, reload, optionsForTask };
}
