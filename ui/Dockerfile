# Web image: the built SPA, served by nginx. It knows nothing about the API.
FROM node:22-alpine AS build

WORKDIR /ui
COPY package.json package-lock.json ./
RUN npm ci
COPY . .

# A checkout with a restrictive umask (750 dirs) would otherwise 403 in the
# runtime stage: nginx runs as uid 101 and needs read/traverse, and it is not
# the owner, so it cannot chmod. The build stage is root, so fix it here.
RUN npm run build && chmod -R a+rX dist

FROM nginxinc/nginx-unprivileged:1.31-alpine
COPY nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=build --chown=101:0 /ui/dist /usr/share/nginx/html

EXPOSE 8080
