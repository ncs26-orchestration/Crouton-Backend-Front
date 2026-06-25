# syntax=docker/dockerfile:1.7

FROM node:24-alpine AS base
RUN apk add --no-cache nodejs npm && npm install -g pnpm@10
WORKDIR /repo

FROM base AS deps
COPY pnpm-workspace.yaml package.json pnpm-lock.yaml* ./
COPY packages/tsconfig/package.json packages/tsconfig/
COPY apps/web/package.json apps/web/
RUN --mount=type=cache,id=pnpm,target=/root/.local/share/pnpm/store \
    pnpm install --frozen-lockfile || pnpm install

FROM deps AS build
COPY packages/tsconfig packages/tsconfig
COPY apps/web apps/web
RUN pnpm --filter web build

FROM nginx:alpine AS runtime
COPY apps/web/nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=build /repo/apps/web/dist /usr/share/nginx/html
EXPOSE 80
CMD ["nginx", "-g", "daemon off;"]
