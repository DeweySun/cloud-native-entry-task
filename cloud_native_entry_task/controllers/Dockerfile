FROM golang:1.26 AS builder

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/dbcp-controller ./cloud_native_entry_task/controllers/cmd/dbcp-controller

FROM alpine:3.22

RUN adduser -D -H -u 10001 controller
COPY --from=builder /out/dbcp-controller /usr/local/bin/dbcp-controller

USER 10001
ENTRYPOINT ["/usr/local/bin/dbcp-controller"]
