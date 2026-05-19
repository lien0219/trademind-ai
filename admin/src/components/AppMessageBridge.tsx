import { App, message as staticMessage } from 'antd';
import type { MessageInstance } from 'antd/es/message/interface';
import { useLayoutEffect } from 'react';

const MESSAGE_METHODS = ['info', 'success', 'error', 'warning', 'loading', 'open', 'destroy'] as const;

type MessageMethod = (typeof MESSAGE_METHODS)[number];

function patchStaticMessage(instance: MessageInstance): () => void {
  const restored = new Map<MessageMethod, MessageInstance[MessageMethod]>();
  for (const key of MESSAGE_METHODS) {
    const orig = staticMessage[key].bind(staticMessage) as MessageInstance[MessageMethod];
    restored.set(key, orig);
    staticMessage[key] = ((...args: Parameters<MessageInstance[MessageMethod]>) =>
      instance[key](...args)) as MessageInstance[MessageMethod];
  }
  return () => {
    for (const key of MESSAGE_METHODS) {
      const orig = restored.get(key);
      if (orig) {
        staticMessage[key] = orig;
      }
    }
  };
}

/** Patches antd static `message.*` to use App context (theme + cssVar). Mount inside `<App>`. */
export default function AppMessageBridge() {
  const { message } = App.useApp();
  useLayoutEffect(() => patchStaticMessage(message), [message]);
  return null;
}
