# Admin — build with pnpm workspace, serve dist with nginx (proxies /api /static to backend).
FROM node:20-bookworm AS builder

WORKDIR /workspace

RUN corepack enable && corepack prepare pnpm@9.15.4 --activate

COPY package.json pnpm-lock.yaml pnpm-workspace.yaml .npmrc ./
COPY admin ./admin
COPY collector ./collector

RUN pnpm install --frozen-lockfile

RUN pnpm --filter @trademind/admin build

FROM nginx:1.31-alpine AS runtime

COPY admin/nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=builder /workspace/admin/dist /usr/share/nginx/html

EXPOSE 80
