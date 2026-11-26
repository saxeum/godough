# API image: the Go server only. The UI lives in ui/Dockerfile.
FROM golang:1.27-alpine AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

# Everything, not a list of directories. The list was `cmd` only, and the first
# package outside it (internal/transport, internal/service) broke the image
# build while the host build stayed green. .dockerignore is what keeps the
# context small.
COPY . .

# Static, no libc, no shell: pgx is pure Go, so CGO is not needed.
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/godough ./cmd/web

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/godough /godough
EXPOSE 4000
ENTRYPOINT ["/godough"]
