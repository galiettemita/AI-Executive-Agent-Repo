# =============================================================================
# Stage 1 — builder
# Install all dependencies (including devDependencies for tsc), then compile.
# =============================================================================
FROM node:22-alpine AS builder
WORKDIR /app

COPY package*.json ./
RUN npm ci --include=dev

COPY . .
RUN npm run build

# =============================================================================
# Stage 2 — runtime
# Copy only compiled output and runtime dependencies.
# =============================================================================
FROM node:22-alpine AS runtime
WORKDIR /app

ENV NODE_ENV=production

COPY --from=builder /app/dist        ./dist
COPY --from=builder /app/node_modules ./node_modules
COPY --from=builder /app/package.json ./package.json

EXPOSE 18089

CMD ["node", "dist/server.js"]
