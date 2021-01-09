FROM golang:1.15-buster

RUN apt-get update &&  DEBIAN_FRONTEND=noninteractive apt-get install -y -q \
	gcc libgtk-3-dev libappindicator3-dev \
 && apt-get clean \
 && rm -rf /var/lib/apt/lists/*

WORKDIR /build
COPY go.* /build/
RUN go mod download

COPY . /build/
