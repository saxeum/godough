# Web image: the built SPA, served by nginx. It knows nothing about the API.
FROM node:26-alpine AS build

WORKDIR /ui
COPY package.json package-lock.json ./
RUN npm ci
COPY . .

# A checkout with a restrictive umask (750 dirs) would otherwise 403 in the
# runtime stage: nginx runs as uid 101 and needs read/traverse, and it is not
# the owner, so it cannot chmod. The build stage is root, so fix it here.
RUN npm run build && chmod -R a+rX dist

FROM nginxinc/nginx-unprivileged:1.31.7-alpine3.24
COPY nginx.conf /etc/nginx/conf.d/default.conf
# The published image lags the Alpine repository — a stock base is where almost
# every finding in `make vuln-images` came from. Patch its packages at build
# time, then drop back to the unprivileged user it ships with.
USER root
RUN apk upgrade --no-cache
USER 101

COPY --from=build --chown=101:0 /ui/dist /usr/share/nginx/html

EXPOSE 8080
