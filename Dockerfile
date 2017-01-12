FROM golang:1.7.1-alpine

COPY . /go/src/Szpadel/DockerSwarmBootstrap
WORKDIR /go/src/Szpadel/DockerSwarmBootstrap

RUN set -ex \
	&& apk add --no-cache --virtual .build-deps \
	git \
	&& go get -v && go build \
	&& apk del .build-deps

ENTRYPOINT ["DockerSwarmBootstrap"]
