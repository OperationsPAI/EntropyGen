# Stage 1: Build
FROM node:25-alpine AS builder
WORKDIR /app

COPY frontend/package.json frontend/package-lock.json ./
RUN npm install --legacy-peer-deps

# Copy OpenAPI spec needed by @hey-api/openapi-ts code generation
COPY docs/openapi.json /docs/openapi.json

COPY frontend/ .
RUN npm run build

# Stage 2: Serve
FROM nginx:alpine
COPY frontend/nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=builder /app/dist /usr/share/nginx/html
EXPOSE 80
CMD ["nginx", "-g", "daemon off;"]
