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
#
# The migrator ships in the same image so the Kubernetes migration Job (#5) can
# run the digest that is deployed rather than a second image that can drift from
# it. Different entrypoint, same artifact.
#
# tlscheck ships too: verifying the CA bundle and the server certificate from
# inside the VPC is a check that has to run where the database is reachable, and
# a CronJob running this image is how #5 wants it alerted (#116).
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/godough ./cmd/web && \
    CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/migrate ./cmd/migrate && \
    CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/tlscheck ./cmd/tlscheck

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/godough /godough
COPY --from=build /out/migrate /migrate
COPY --from=build /out/tlscheck /tlscheck
EXPOSE 4000
ENTRYPOINT ["/godough"]
