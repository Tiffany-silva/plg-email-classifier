# syntax=docker/dockerfile:1

# ---- Build stage ----
FROM golang:1.22 AS build
WORKDIR /src

# This module has no external dependencies, so go.mod alone is enough.
COPY go.mod ./
COPY . .

# Static binary: CGO off so it runs on the minimal distroless base and uses
# Go's pure-Go DNS resolver (the correct choice inside containers).
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/server ./cmd/server

# ---- Runtime stage ----
# distroless static: tiny image with no shell — minimal CVE surface for Choreo's
# Trivy scan.
FROM gcr.io/distroless/static-debian12:latest AS runtime
WORKDIR /app

# Binary and seed list, owned by the non-root UID below so the background
# refresher can rewrite the list file when DISPOSABLE_LIST_PATH points at it.
COPY --from=build --chown=10014:10014 /out/server /app/server
COPY --chown=10014:10014 data/ /app/data/

# Choreo requires the container to run as a non-root user whose UID is a numeric
# value in the range 10000-20000. distroless needs no /etc/passwd entry for a
# numeric UID to be valid.
USER 10014

EXPOSE 8080
ENTRYPOINT ["/app/server"]
