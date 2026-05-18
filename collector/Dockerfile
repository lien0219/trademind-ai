# Collector — Playwright base image + pnpm workspace build (needs admin package.json for lockfile).
FROM mcr.microsoft.com/playwright:v1.60.0-jammy

USER root

WORKDIR /workspace

RUN apt-get update \
  && apt-get install -y --no-install-recommends curl \
  && rm -rf /var/lib/apt/lists/*

RUN corepack enable && corepack prepare pnpm@9.15.4 --activate

COPY package.json pnpm-lock.yaml pnpm-workspace.yaml .npmrc ./
COPY admin ./admin
COPY collector ./collector

RUN pnpm install --frozen-lockfile

RUN pnpm --filter @trademind/collector build

ENV NODE_ENV=production

EXPOSE 3001

CMD ["pnpm", "--filter", "@trademind/collector", "start"]
