# Build context: deploy/nginx
# nginx with the reverse-proxy config baked in, so the VM needs no config files.
FROM nginx:1.27-alpine
COPY nginx.conf /etc/nginx/conf.d/default.conf
