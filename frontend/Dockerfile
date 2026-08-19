# ─── Stage 1: build ──────────────────────────────────────────────────────────
FROM node:22-alpine@sha256:16e22a550f3863206a3f701448c45f7912c6896a62de43add43bb9c86130c3e2 AS builder

WORKDIR /app

# Install dependencies first to leverage layer caching.
COPY package*.json ./
RUN npm ci

# Copy the rest of the frontend source and build.
COPY ./ ./
RUN npm run build
# Vite outputs to /app/dist by default.

# ─── Stage 2: serve ──────────────────────────────────────────────────────────
FROM nginx:alpine@sha256:4a73073bd557c65b759505da037898b61f1be6cbcc3c2c3aeac22d2a470c1752 AS final

# Remove the default nginx welcome page.
RUN rm -rf /usr/share/nginx/html/*

# Copy built assets.
COPY --from=builder /app/dist /usr/share/nginx/html

# SPA-friendly nginx config: serve index.html for all unknown paths so that
# client-side routing (React Router / TanStack Router) works correctly.
RUN printf 'server {\n\
    listen       80;\n\
    server_name  _;\n\
    root         /usr/share/nginx/html;\n\
    index        index.html;\n\
    \n\
    # Serve pre-built assets directly; fall back to index.html for SPA routes.\n\
    location / {\n\
    try_files $uri $uri/ /index.html;\n\
    }\n\
    \n\
    # Cache static assets aggressively; versions are content-hashed by Vite.\n\
    location ~* \\.(js|css|woff2?|ttf|eot|svg|png|ico)$ {\n\
    expires 1y;\n\
    add_header Cache-Control "public, immutable";\n\
    try_files $uri =404;\n\
    }\n\
    \n\
    # Do not cache index.html so browsers always pick up new asset hashes.\n\
    location = /index.html {\n\
    add_header Cache-Control "no-store";\n\
    }\n\
    }\n' > /etc/nginx/conf.d/default.conf

EXPOSE 80

CMD ["nginx", "-g", "daemon off;"]
