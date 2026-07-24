FROM node:24.13.0-alpine3.23 AS build
ENV PNPM_HOME=/pnpm
ENV PATH=$PNPM_HOME:$PATH
RUN corepack enable
WORKDIR /src

COPY package.json pnpm-lock.yaml pnpm-workspace.yaml .npmrc ./
COPY apps/web/package.json ./apps/web/package.json
COPY packages/contracts/package.json ./packages/contracts/package.json
RUN pnpm install --frozen-lockfile

COPY apps/web ./apps/web
COPY packages/contracts ./packages/contracts
ARG NEXT_PUBLIC_API_URL=http://localhost:8080
ENV NEXT_PUBLIC_API_URL=$NEXT_PUBLIC_API_URL
RUN pnpm --filter @glazz/web build

FROM node:24.13.0-alpine3.23
ENV NODE_ENV=production
ENV HOSTNAME=0.0.0.0
ENV PORT=3000
RUN addgroup -S glazz && adduser -S -G glazz glazz
WORKDIR /app
COPY --from=build --chown=glazz:glazz /src/apps/web/.next/standalone ./
COPY --from=build --chown=glazz:glazz /src/apps/web/.next/static ./apps/web/.next/static
USER glazz
EXPOSE 3000
CMD ["node", "apps/web/server.js"]
