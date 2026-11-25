# API image: the Go server only. The UI lives in ui/Dockerfile.
FROM golang:1.27-alpine AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd ./cmd

# Static, no libc, no shell: pgx is pure Go, so CGO is not needed.
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/godough ./cmd/web

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/godough /godough
EXPOSE 4000
ENTRYPOINT ["/godough"]
