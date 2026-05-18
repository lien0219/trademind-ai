import path from 'node:path';
import { fileURLToPath } from 'node:url';

const __dirname = path.dirname(fileURLToPath(import.meta.url));

/** Monorepo root (contains package.json, docker-compose.yml). */
export const repoRoot = path.resolve(__dirname, '..', '..');

export const backendDir = path.join(repoRoot, 'backend');
