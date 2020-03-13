FROM golang:1.13 as builder

ARG version
ARG builddate

WORKDIR /src

# hadolint ignore=SC2097,SC2098
ENV GOOS=linux \
    GOARCH=amd64 \
    CGO_ENABLED=0 \
    GOPROXY=

COPY go.mod .
COPY go.sum .

RUN go mod download

COPY . .

RUN go build \
    -a \
    -tags netgo \
    -ldflags "-s -w -X main.Version=${version} -X main.BuildDate=${builddate}" \
    -o /go/bin/covina && \
  strip /go/bin/covina

FROM gcr.io/distroless/base:3c29f81d9601750a95140e4297a06765c41ad10e

EXPOSE 8002
COPY --from=builder /go/bin/covina /app/covina

LABEL works.devops.type="covina"
LABEL works.devops.group="services"
LABEL works.devops.version=${version}
LABEL works.devops.date=${builddate}

CMD ["/app/covina"]