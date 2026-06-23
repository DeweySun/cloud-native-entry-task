FROM golang:1.26 AS builder

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/tcpd ./cmd/tcpd \
    && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/httpd ./cmd/httpd \
    && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/migrate ./cmd/migrate \
    && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/seed ./cmd/seed

FROM nginx:1.31-alpine

WORKDIR /app

RUN apk add --no-cache ca-certificates

COPY --from=builder /out/tcpd /app/bin/tcpd
COPY --from=builder /out/httpd /app/bin/httpd
COPY --from=builder /out/migrate /app/bin/migrate
COPY --from=builder /out/seed /app/bin/seed
COPY config/config.example.json /app/config/config.json
COPY db/schema.sql /app/db/schema.sql
COPY web/static /app/web/static
COPY cloud_native_entry_task/CRD/docker/nginx.conf /etc/nginx/nginx.conf
COPY cloud_native_entry_task/CRD/docker/entrypoint.sh /app/entrypoint.sh

RUN chmod +x /app/entrypoint.sh \
    && mkdir -p /app/runtime/profile-pictures /app/runtime/logs

EXPOSE 8080

ENTRYPOINT ["/app/entrypoint.sh"]

