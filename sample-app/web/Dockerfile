FROM node:24.11.1-bookworm-slim
WORKDIR /app
COPY package.json package.json
RUN npm install --before=2025-11-21T00:00:00.000Z
COPY . .
EXPOSE 5173
CMD ["npm", "run", "dev"]
