FROM mcr.microsoft.com/playwright:v1.44.0-jammy

WORKDIR /app

COPY package*.json ./
RUN npm ci --omit=dev

COPY tsconfig.json ./
COPY src ./src
RUN npm run build

RUN npx playwright install chromium

ENV BREVIO_ENV=production
ENV BROWSER_MCP_PORT=7788

EXPOSE 7788

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
  CMD curl -f http://localhost:7788/health || exit 1

USER pwuser

CMD ["node", "dist/server.js"]
