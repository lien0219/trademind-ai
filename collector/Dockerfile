# Build the workspace, then deploy only Collector's production graph and dist.
FROM node:24-bookworm-slim AS builder

WORKDIR /workspace

RUN npm install --global pnpm@10.34.6

COPY package.json pnpm-lock.yaml pnpm-workspace.yaml .npmrc ./
COPY admin ./admin
COPY collector ./collector
COPY scripts/patch-pro-field-antd-select.mjs ./scripts/

RUN pnpm install --frozen-lockfile

RUN pnpm --filter @trademind/collector build

RUN pnpm --filter @trademind/collector deploy --prod --legacy /out/collector

FROM node:24-bookworm-slim AS runtime

WORKDIR /workspace/collector

COPY --from=builder /out/collector/ ./

ENV NODE_ENV=production
ENV PLAYWRIGHT_BROWSERS_PATH=/ms-playwright

RUN apt-get update \
  && apt-get install -y --no-install-recommends fonts-noto-cjk \
  && ./node_modules/.bin/playwright install --with-deps chromium \
  && mkdir -p /workspace/data/browser-profiles /workspace/data/storage-states \
  && chown -R node:node /workspace/data /workspace/collector /ms-playwright \
  && rm -rf /var/lib/apt/lists/*

EXPOSE 3001

USER node

CMD ["node", "dist/index.js"]
