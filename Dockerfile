FROM golang:1.21-alpine as builder
LABEL maintainer="Hetao<hetao@hetao.name>"
LABEL version="1.0"

WORKDIR /data/ip-location-qqwry/

COPY . .  
RUN sed -i 's/dl-cdn.alpinelinux.org/mirrors.aliyun.com/g' /etc/apk/repositories \
	&& apk update && apk add git tree \
	&& tree -L 3 
RUN export GOPROXY=https://goproxy.cn && go build -o bin/ip-location-qqwry && rm -rf /var/lib/apk/*

FROM alpine:3.14 as prod

RUN apk --no-cache add ca-certificates

WORKDIR /data/ip-location-qqwry/

RUN mkdir bin/

COPY qqwry.dat/20231122/qqwry.dat /data/ip-location-qqwry/bin/
COPY --from=0 /data/ip-location-qqwry/bin/ip-location-qqwry bin/

HEALTHCHECK --interval=5s --timeout=5s --retries=3 \
    CMD ps aux | grep "ip-location-qqwry" | grep -v "grep" > /dev/null; if [ 0 != $? ]; then exit 1; fi

CMD ["/data/ip-location-qqwry/bin/ip-location-qqwry", "-b" , "0.0.0.0:80"]
