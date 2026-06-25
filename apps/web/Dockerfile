# Build context: repo root (needs the workspace lockfile)
FROM node:26-alpine AS build
WORKDIR /repo
RUN npm install -g pnpm@9.15.0
COPY pnpm-workspace.yaml package.json pnpm-lock.yaml ./
COPY apps/web/package.json apps/web/package.json
RUN pnpm install --frozen-lockfile --filter web...
COPY apps/web apps/web
RUN pnpm --filter web build

FROM node:26-alpine AS runtime
WORKDIR /app
ENV NODE_ENV=production
ENV PORT=3000
# Nitro's node-server output is self-contained (deps bundled into .output).
COPY --from=build /repo/apps/web/.output ./.output
EXPOSE 3000
USER node
CMD ["node", ".output/server/index.mjs"]
