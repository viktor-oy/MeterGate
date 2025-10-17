FROM node:24.11.1-bookworm-slim AS base
WORKDIR /app
ENV NODE_ENV=production

FROM base AS deps
COPY package.json package-lock.json ./
COPY prisma prisma
RUN npm ci --include=dev

FROM deps AS build
COPY tsconfig.json tsconfig.build.json nest-cli.json ./
COPY src src
RUN npm run prisma:generate && npm run build

FROM base AS runtime
COPY package.json package-lock.json ./
COPY prisma prisma
COPY infra infra
COPY --from=deps /app/node_modules node_modules
COPY --from=build /app/dist dist
RUN mkdir -p /app/otel-output
EXPOSE 3000
CMD ["node", "dist/src/controlplane/main.js"]
