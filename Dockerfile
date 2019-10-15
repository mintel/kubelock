FROM golang:1.12.9-alpine3.10 as builder

RUN apk add --no-cache git

ENV GO111MODULE=on
WORKDIR /app
COPY go.mod .
COPY go.sum .
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build .

FROM scratch
COPY --from=builder /app/kubelock /usr/local/bin/kubelock

ENTRYPOINT ["kubelock"]
