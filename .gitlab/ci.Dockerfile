FROM ubuntu:22.04

ARG GO_VERSION=1.22.7
ARG GO_ARCH=amd64
ARG MAGE_VERSION=1.15.0

ENV PATH=$PATH:/usr/local/go/bin:~/go/bin

RUN apt-get -q -y update  \
    && apt-get -y install build-essential ca-certificates curl gcc gnupg2 libssl-dev make software-properties-common zip

RUN mkdir -p /etc/apt/keyrings \
    && curl -fsSL https://download.docker.com/linux/ubuntu/gpg -o /etc/apt/keyrings/docker.asc \
    && chmod a+r /etc/apt/keyrings/docker.asc \
    && echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.asc] \
        https://download.docker.com/linux/ubuntu $(. /etc/os-release && echo \"$VERSION_CODENAME\") stable" \
        | tee /etc/apt/sources.list.d/docker.list > /dev/null \
    && apt-get update \
    && apt-get install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin

RUN  curl -s -O -L https://golang.org/dl/go${GO_VERSION}.linux-${GO_ARCH}.tar.gz \
     && rm -rf /usr/local/go \
     && tar -C /usr/local -xzf go${GO_VERSION}.linux-${GO_ARCH}.tar.gz \
     && rm -f go${GO_VERSION}.linux-${GO_ARCH}.tar.gz

RUN go install github.com/magefile/mage@v${MAGE_VERSION}

RUN curl -fsSL https://deb.nodesource.com/setup_22.x | bash - \
    && apt-get -y install nodejs

