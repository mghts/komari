FROM node:22-alpine@sha256:c610fcdfb1d5b4740dd70c284ed3cb16bb857e0f7166196e36a5501df7a3aa32 AS frontend
RUN apk add --no-cache git
COPY build/frontend.json /tmp/frontend.json
COPY build/agent.json /tmp/agent.json
WORKDIR /frontend
RUN REPO="$(node -p 'require("/tmp/frontend.json").repository')" && \
    REF="$(node -p 'require("/tmp/frontend.json").commit')" && \
    test "${#REF}" = 40 && \
    git init && git remote add origin "https://github.com/${REPO}.git" && \
    git fetch --depth=1 origin "$REF" && git checkout --detach FETCH_HEAD && \
    test "$(git rev-parse HEAD)" = "$REF" && \
    npm ci --no-audit --no-fund && \
    npm run check:fork -- /tmp/agent.json && \
    SOURCE_DATE_EPOCH="$(git show -s --format=%ct HEAD)" npm run build

FROM golang:1.25-alpine@sha256:1ae0735f00daffa3aaf1363a5184c0d2dc55c78e3db4ec70241cdac97bf84b59 AS source
RUN apk add --no-cache build-base
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download && go mod verify
COPY . .
COPY --from=frontend /frontend/dist ./web/public/defaultTheme/dist
COPY --from=frontend /frontend/komari-theme.json ./web/public/defaultTheme/komari-theme.json

FROM source AS test
RUN CGO_ENABLED=1 go test ./...

FROM source AS build
ARG VERSION=0.0.0-dev
ARG REVISION=unknown
RUN CGO_ENABLED=1 go build -trimpath -buildvcs=false -ldflags="-s -w -linkmode external -extldflags '-static' -X github.com/komari-monitor/komari/utils.CurrentVersion=${VERSION} -X github.com/komari-monitor/komari/utils.VersionHash=${REVISION}" -o /out/komari .

FROM alpine:3.21@sha256:48b0309ca019d89d40f670aa1bc06e426dc0931948452e8491e3d65087abc07d
RUN apk add --no-cache ca-certificates curl tzdata
WORKDIR /app
ARG VERSION=0.0.0-dev
ARG REVISION=unknown
LABEL org.opencontainers.image.source="https://github.com/mghts/komari" \
      org.opencontainers.image.version="${VERSION}" \
      org.opencontainers.image.revision="${REVISION}" \
      org.opencontainers.image.licenses="MIT"
COPY --from=build /out/komari /app/komari
ENV GIN_MODE=release
ENV KOMARI_LISTEN=0.0.0.0:25774
EXPOSE 25774
HEALTHCHECK --interval=30s --timeout=5s --start-period=30s CMD curl -fsS http://127.0.0.1:25774/ >/dev/null || exit 1
CMD ["/app/komari", "server"]
