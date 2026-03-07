# ── Stage 1: build ───────────────────────────────────────────────────────────
FROM node:22-alpine AS builder

WORKDIR /app

# Install dependencies first (cache layer)
COPY frontend/package.json frontend/package-lock.json* ./
RUN npm ci --ignore-scripts

# Copy source and build
COPY frontend/ .

RUN npm run build

# ── Stage 2: serve ────────────────────────────────────────────────────────────
FROM nginx:1.27-alpine AS runtime

# envsubst is part of gettext — needed to render ${WEBAPI_URL} in the template.
RUN apk add --no-cache gettext

# Remove default nginx static assets
RUN rm -rf /usr/share/nginx/html/*

# Copy built assets from builder stage
COPY --from=builder /app/dist /usr/share/nginx/html

# Copy the nginx config as a template — ${WEBAPI_URL} will be substituted at
# container startup by envsubst, so the webapi address can be set via env var
# without rebuilding the image.
COPY dev/nginx.conf /etc/nginx/nginx.conf.template

# Default webapi URL: the Helm-installed ClusterIP service in nebari-system.
# Override with -e WEBAPI_URL=http://... when starting the container standalone.
ENV WEBAPI_URL=http://nebari-landing-webapi.nebari-system.svc.cluster.local:8080

EXPOSE 8080

HEALTHCHECK --interval=15s --timeout=3s --start-period=5s --retries=3 \
  CMD wget -qO- http://localhost:8080/healthz || exit 1

# 1. Substitute ${WEBAPI_URL} in the template and write the final nginx.conf.
#    Only ${WEBAPI_URL} is substituted — all other nginx $variable references
#    (e.g. $remote_addr, $http_authorization) are left untouched.
# 2. Start nginx in the foreground.
CMD ["/bin/sh", "-c", \
  "envsubst '${WEBAPI_URL}' < /etc/nginx/nginx.conf.template > /etc/nginx/nginx.conf && nginx -g 'daemon off;'"]
