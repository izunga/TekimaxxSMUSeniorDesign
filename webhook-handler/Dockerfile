<<<<<<< Updated upstream
FROM node:20-slim

RUN groupadd --gid 1001 appgroup && useradd --uid 1001 --gid appgroup --no-create-home appuser
=======
FROM node:20-alpine
>>>>>>> Stashed changes

WORKDIR /app

COPY package*.json ./
<<<<<<< Updated upstream
RUN npm ci

COPY tsconfig.json ./
COPY src/ ./src/

RUN npx tsc && mkdir -p /var/lib/tekimax && chown -R appuser:appgroup /app /var/lib/tekimax

USER appuser
ENV EVENT_STORE_PATH=/var/lib/tekimax/stripe-events.json
EXPOSE 3001
CMD ["node", "dist/index.js"]
=======
RUN npm install

COPY . .

RUN npm run build

EXPOSE 3000

CMD ["npm", "start"]
>>>>>>> Stashed changes
