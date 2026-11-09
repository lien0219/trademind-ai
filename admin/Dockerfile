# Admin — build with pnpm workspace, serve dist with nginx (proxies /api /static to backend).
FROM node:25-bookworm AS builder

WORKDIR /workspace

RUN npm install --global pnpm@10.34.6

COPY package.json pnpm-lock.yaml pnpm-workspace.yaml .npmrc ./
COPY admin ./admin
COPY collector ./collector
COPY scripts/patch-pro-field-antd-select.mjs ./scripts/

RUN pnpm install --frozen-lockfile

RUN pnpm --filter @trademind/admin build

FROM nginx:1.31.7-alpine AS runtime

# Apply upstream Alpine security updates before serving the static admin bundle.
# Fail the build if the repository lacks the fixes required by the image gate.
RUN apk upgrade --no-cache \
    && apk add --no-cache --upgrade \
        'brotli-libs>=1.2.1-r0' \
        'curl>=8.23.0-r0' \
        'libcurl>=8.23.0-r0' \
        libexpat libx11

COPY admin/nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=builder /workspace/admin/dist /usr/share/nginx/html

EXPOSE 80
