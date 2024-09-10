FROM ubuntu:jammy

RUN apt-get -q update \
  && DEBIAN_FRONTEND=noninteractive apt-get -q -y install \
  libnginx-mod-rtmp

COPY rtmp.conf /etc/nginx/conf.d
