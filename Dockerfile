FROM --platform=$BUILDPLATFORM golang:1.25-alpine AS builder
LABEL maintainer="flyzstu <flyzstu@gmail.com>"
LABEL org.opencontainers.image.title="xhttp-box"
LABEL org.opencontainers.image.source="https://github.com/flyzstu/xhttp-box"
COPY . /go/src/github.com/sagernet/sing-box
WORKDIR /go/src/github.com/sagernet/sing-box
ARG TARGETOS TARGETARCH
ARG GOPROXY=""
ENV GOPROXY ${GOPROXY}
ENV CGO_ENABLED=0
ENV GOOS=$TARGETOS
ENV GOARCH=$TARGETARCH
RUN set -ex \
    && apk add git build-base \
    && export COMMIT=$(git rev-parse --short HEAD) \
    && export VERSION=$(go run ./cmd/internal/read_tag) \
    && export TAGS=$(cat release/DEFAULT_BUILD_TAGS_OTHERS) \
    && export LDFLAGS_SHARED=$(cat release/LDFLAGS) \
    && go build -v -trimpath -tags "$TAGS" \
        -o /go/bin/xhttp-box \
        -ldflags "-X \"github.com/sagernet/sing-box/constant.Version=$VERSION\" $LDFLAGS_SHARED -s -w -buildid=" \
        ./cmd/sing-box
FROM --platform=$TARGETPLATFORM alpine AS dist
LABEL maintainer="flyzstu <flyzstu@gmail.com>"
LABEL org.opencontainers.image.title="xhttp-box"
LABEL org.opencontainers.image.source="https://github.com/flyzstu/xhttp-box"
RUN set -ex \
    && apk add --no-cache --upgrade bash tzdata ca-certificates nftables
COPY --from=builder /go/bin/xhttp-box /usr/local/bin/xhttp-box
ENTRYPOINT ["xhttp-box"]
