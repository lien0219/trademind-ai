# Admin — build with pnpm workspace, serve dist with nginx (proxies /api /static to backend).
FROM node:24-bookworm AS builder

WORKDIR /workspace

RUN npm install --global pnpm@10.34.6

COPY package.json pnpm-lock.yaml pnpm-workspace.yaml .npmrc ./
COPY admin ./admin
COPY collector ./collector
COPY scripts/patch-pro-field-antd-select.mjs ./scripts/

RUN pnpm install --frozen-lockfile

RUN pnpm --filter @trademind/admin build

FROM nginx:1.31.7-alpine AS runtime

RUN apk add --no-cache --upgrade libexpat libx11

COPY admin/nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=builder /workspace/admin/dist /usr/share/nginx/html

EXPOSE 80
