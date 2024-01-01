init:
	git submodule update
build:
	export GOPROXY=https://goproxy.cn && go build -o bin/ip-location-qqwry
docker-image:
	DOCKER_BUILDKIT=1 docker build -t hetao29/ip-location-qqwry .
docker-push:
	docker push hetao29/ip-location-qqwry:latest
