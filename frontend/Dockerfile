# ─── Stage 1: build ──────────────────────────────────────────────────────────
FROM node:22-alpine AS builder

WORKDIR /app

# Install dependencies first to leverage layer caching.
COPY package*.json ./
RUN npm ci

# Copy the rest of the frontend source and build.
COPY ./ ./
RUN npm run build
# Vite outputs to /app/dist by default.

# ─── Stage 2: serve ──────────────────────────────────────────────────────────
# nginx:alpine for production static file serving — efficient, hardened, tiny.
# The nginx.conf (injected by Helm at deploy time via ConfigMap) handles:
#   - SPA fallback (try_files → index.html)
#   - /api/* proxy_pass to the webapi ClusterIP with Authorization header
#   - aggressive Cache-Control for content-hashed assets
FROM nginx:alpine AS final

# Remove the default nginx welcome page.
RUN rm -rf /usr/share/nginx/html/*

# Copy built assets. nginx.conf is NOT baked in — it is injected at runtime
# by the Helm ConfigMap (volumeMount over /etc/nginx/nginx.conf) so the image
# stays config-agnostic.
COPY --from=builder /app/dist /usr/share/nginx/html

EXPOSE 80

CMD ["nginx", "-g", "daemon off;"]
