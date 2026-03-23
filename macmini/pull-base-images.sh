#!/bin/bash
set -e

MIRROR="docker.m.daocloud.io/library"

echo "拉取基础镜像..."

docker pull ${MIRROR}/python:3.12-slim
docker tag  ${MIRROR}/python:3.12-slim python:3.12-slim

docker pull ${MIRROR}/python:3.10-slim
docker tag  ${MIRROR}/python:3.10-slim python:3.10-slim

docker pull ${MIRROR}/alpine:3.19
docker tag  ${MIRROR}/alpine:3.19 alpine:3.19

docker pull ${MIRROR}/alpine:3.20
docker tag  ${MIRROR}/alpine:3.20 alpine:3.20

docker pull docker.m.daocloud.io/cloudflare/cloudflared:latest
docker tag  docker.m.daocloud.io/cloudflare/cloudflared:latest cloudflare/cloudflared:latest

echo "全部拉取完成，可以开始 build 了"
