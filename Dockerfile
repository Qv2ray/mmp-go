FROM mzz2017/git:alpine AS version
WORKDIR /build
ADD .git ./.git
RUN git describe --abbrev=0 --tags > ./version

FROM golang:1.16-alpine AS builder
WORKDIR /build
ADD . .
ENV GO111MODULE=on
ENV GOPROXY=https://goproxy.io
COPY --from=version /build/version ./
RUN export VERSION=$(cat ./version) && CGO_ENABLED=0 go build -ldflags '-X github.com/Qv2ray/mmp-go/config.Version=${VERSION} -s -w -extldflags "-static"' -o mmp-go .

FROM alpine
COPY --from=builder /build/mmp-go /usr/bin/
VOLUME /etc/mmp-go
ENTRYPOINT ["mmp-go", "-conf", "/etc/mmp-go/config.json"]
