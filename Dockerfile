FROM golang:1.26-alpine AS builder

RUN apk add --no-cache git jq

ENV GO111MODULE=on
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /usr/local/bin/kubelock .

FROM scratch
COPY --from=builder /usr/local/bin/kubelock /usr/local/bin/kubelock

ENTRYPOINT ["kubelock"]
