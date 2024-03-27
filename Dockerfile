FROM golang:1.22

WORKDIR /app

COPY ./go.mod ./go.sum ./


RUN go mod download

COPY *.go ./

# Copy temp config file for testing
COPY ./config.yaml ./

RUN go build -o /gatekeeper-advanced

CMD ["/gatekeeper-advanced"]