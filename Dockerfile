# Dockerfile for the boxpackerclient HTTP service.
#
# Builds from this directory alone; the boxpacker dependency is fetched from
# GitHub (github.com/ryanlholt/BoxPacker).
#
#   docker build -t boxpackerclient .
#   docker run --rm -p 8080:8080 -e BOXPACKER_API_TOKEN=your-secret boxpackerclient
#   curl -s -H "Authorization: Bearer your-secret" \
#     --data-binary @example.json localhost:8080/pack
#
# The HTTP service refuses to start without BOXPACKER_API_TOKEN. Pass secrets at
# runtime with -e (never bake them into the image): BOXPACKER_API_TOKEN is
# required; BOXPACKER_CF_SECRET enables the Cloudflare X-Origin-Auth check.

# ---- build stage ----
FROM golang:1.26-alpine AS build

# git for module downloads; CA certs for TLS.
RUN apk add --no-cache git ca-certificates

WORKDIR /src

# Download dependencies first for better layer caching.
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/boxpackerclient .

# ---- runtime stage ----
FROM gcr.io/distroless/static-debian12 AS runtime

COPY --from=build /out/boxpackerclient /usr/local/bin/boxpackerclient

EXPOSE 8080
# Listen on all interfaces inside the container; override flags via `docker run`.
ENTRYPOINT ["/usr/local/bin/boxpackerclient"]
CMD ["-http", ":8080"]
