# Stage 1: Build frontend assets
FROM node:20-alpine AS builder
WORKDIR /app
COPY package*.json ./
RUN npm install
COPY . .
RUN npm run build

# Stage 2: Serve static assets and proxy API requests
FROM nginx:stable-alpine
COPY --from=builder /app/dist /usr/share/nginx/html
COPY nginx.conf.template /etc/nginx/templates/default.conf.template
ENV NGINX_PORT=80
ENV CLIENT_MAX_BODY_SIZE=64m
ENV BACKEND_UPSTREAM=http://127.0.0.1:8080
ENV NGINX_ENVSUBST_FILTER="NGINX_PORT|CLIENT_MAX_BODY_SIZE|BACKEND_UPSTREAM"
EXPOSE 80
CMD ["nginx", "-g", "daemon off;"]
