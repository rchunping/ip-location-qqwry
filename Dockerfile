FROM golang:1.17-alpine as builder
LABEL maintainer="Hetao<hetao@hetao.name>"
LABEL version="1.0"

WORKDIR /data/ipquery/

COPY . .

RUN sed -i 's/dl-cdn.alpinelinux.org/mirrors.aliyun.com/g' /etc/apk/repositories \
	&& apk update && apk add git tree \
	&& tree -L 3 
RUN export GOPROXY=https://goproxy.cn && go build -o bin/ipquery && rm -rf /var/lib/apk/*

FROM alpine:3.14 as prod

RUN apk --no-cache add ca-certificates

WORKDIR /data/ipquery/

RUN mkdir bin/

COPY qqwry.dat  /data/ipquery/
COPY --from=0 /data/ipquery/bin/ipquery bin/

HEALTHCHECK --interval=5s --timeout=5s --retries=3 \
    CMD ps aux | grep "ipquery" | grep -v "grep" > /dev/null; if [ 0 != $? ]; then exit 1; fi

CMD ["/data/ipquery/bin/ipquery", "-b" , "0.0.0.0:80"]
