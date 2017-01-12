#!/usr/bin/env bash
DIR=$( dirname "${BASH_SOURCE[0]}")/../

docker build --pull -t docker-swarm-bootstrap $DIR
docker tag docker-swarm-bootstrap szpadel/docker-swarm-bootstrap
docker push szpadel/docker-swarm-bootstrap
