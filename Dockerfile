FROM golang:1.16.4-alpine3.13 as builder

RUN apk add --no-cache git jq

ENV GO111MODULE=on
WORKDIR /app
COPY go.mod .
COPY go.sum .
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build . && \
    mv kubelock /usr/local/bin/

FROM scratch
COPY --from=builder /usr/local/bin/kubelock /usr/local/bin/kubelock

ENTRYPOINT ["kubelock"]
