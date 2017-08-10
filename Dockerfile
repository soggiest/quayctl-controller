FROM alpine:3.6
MAINTAINER Nicholas Lane "nicklaneovi@gmail.com"

# Install base packages
RUN apk update && apk upgrade && \
    apk add --no-cache bash coreutils && \
    echo -ne "Alpine Linux v3.6 image. (`uname -rsv`)\n" >> /.built && cat /.built

ADD quayctl-controller /

ENTRYPOINT ["/quayctl-controller"]
