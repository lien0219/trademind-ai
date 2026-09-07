# Collector — Playwright base image + pnpm workspace build (needs admin package.json for lockfile).
FROM mcr.microsoft.com/playwright:v1.63.0-jammy

USER root

WORKDIR /workspace

RUN apt-get update \
  && apt-get install -y --no-install-recommends curl \
  && rm -rf /var/lib/apt/lists/*

RUN npm install --global pnpm@9.15.4

COPY package.json pnpm-lock.yaml pnpm-workspace.yaml .npmrc ./
COPY admin ./admin
COPY collector ./collector
COPY scripts/patch-pro-field-antd-select.mjs ./scripts/

RUN pnpm install --frozen-lockfile

RUN pnpm --filter @trademind/collector build

RUN mkdir -p /workspace/data/browser-profiles /workspace/data/storage-states \
  && chown -R pwuser:pwuser /workspace/data

ENV NODE_ENV=production

EXPOSE 3001

USER pwuser

CMD ["pnpm", "--filter", "@trademind/collector", "start"]
