# Multi-stage build. The Go binary is static. Git is copied out of Alpine
# with its musl libraries so the scratch image can still init a workspace.
FROM golang:1.26-alpine AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . ./src
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/symphony ./src/cmd/symphony

RUN set -eu; \
    apk add --no-cache ca-certificates git; \
    mkdir -p /rootfs/tmp /rootfs/data /rootfs/usr/bin /rootfs/usr/libexec /rootfs/usr/share /rootfs/lib /rootfs/usr/lib /rootfs/etc/ssl/certs; \
    cp /out/symphony /rootfs/symphony; \
    chmod 1777 /rootfs/tmp; \
    chown 65532:65532 /rootfs/data; \
    cp /etc/ssl/certs/ca-certificates.crt /rootfs/etc/ssl/certs/ca-certificates.crt; \
    cp -a /usr/bin/git /rootfs/usr/bin/git; \
    cp -a /usr/libexec/git-core /rootfs/usr/libexec/git-core; \
    cp -a /usr/share/git-core /rootfs/usr/share/git-core; \
    ldd /usr/bin/git | awk '/=>/ { print $3 } /^\t\// { print $1 }' | sort -u > /tmp/git-libs; \
    while read -r lib; do \
      if [ -z "$lib" ] || [ ! -e "$lib" ]; then continue; fi; \
      mkdir -p "/rootfs$(dirname "$lib")"; \
      cp -aL "$lib" "/rootfs$lib"; \
    done < /tmp/git-libs

FROM scratch
COPY --from=build /rootfs /
COPY web /web
USER 65532:65532
ENV PATH=/usr/bin \
    HOME=/tmp
ENTRYPOINT ["/symphony"]
CMD ["serve", "-addr", "0.0.0.0:8080", "-web", "/web", "-workspace", "/data"]
