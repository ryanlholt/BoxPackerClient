# Dockerfile for the boxpackerclient HTTP service.
#
# Builds from this directory alone; the boxpacker dependency is fetched from
# GitHub (github.com/ryanlholt/BoxPacker).
#
#   # public dependency:
#   docker build -t boxpackerclient .
#
#   # private dependency (forwards your SSH agent for the module download):
#   docker build --ssh default --build-arg PRIVATE_DEP=1 -t boxpackerclient .
#
#   docker run --rm -p 8080:8080 boxpackerclient
#   curl -s --data-binary @example.json localhost:8080/pack

# ---- build stage ----
FROM golang:1.25-alpine AS build

# git + ssh client for module downloads; CA certs for TLS.
RUN apk add --no-cache git openssh-client ca-certificates

# Treat the dependency as outside the public proxy/checksum DB. Harmless for a
# public repo; required for a private one.
ENV GOPRIVATE=github.com/ryanlholt/*

# For a private dependency, rewrite the GitHub HTTPS URL to SSH so the agent
# forwarded via `--ssh default` can authenticate. Off by default so a plain
# public build uses HTTPS.
ARG PRIVATE_DEP=0
RUN if [ "$PRIVATE_DEP" = "1" ]; then \
      git config --global url."git@github.com:".insteadOf "https://github.com/" && \
      mkdir -p -m 0700 /root/.ssh && \
      ssh-keyscan github.com >> /root/.ssh/known_hosts 2>/dev/null ; \
    fi

WORKDIR /src

# Download dependencies first for better layer caching. (go.* picks up go.sum
# once it exists.) The ssh mount is unused for public builds.
COPY go.* ./
RUN --mount=type=ssh go mod download

COPY . .
RUN --mount=type=ssh CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/boxpackerclient .

# ---- runtime stage ----
FROM gcr.io/distroless/static-debian12 AS runtime

COPY --from=build /out/boxpackerclient /usr/local/bin/boxpackerclient

EXPOSE 8080
# Listen on all interfaces inside the container; override flags via `docker run`.
ENTRYPOINT ["/usr/local/bin/boxpackerclient"]
CMD ["-http", ":8080"]
