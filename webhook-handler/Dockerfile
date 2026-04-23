FROM node:20-slim

RUN groupadd --gid 1001 appgroup && useradd --uid 1001 --gid appgroup --no-create-home appuser

WORKDIR /app

COPY package*.json ./
RUN npm ci

COPY tsconfig.json ./
COPY src/ ./src/

RUN npx tsc && mkdir -p /var/lib/tekimax && chown -R appuser:appgroup /app /var/lib/tekimax

USER appuser
ENV EVENT_STORE_PATH=/var/lib/tekimax/stripe-events.json
EXPOSE 3001
CMD ["node", "dist/index.js"]
