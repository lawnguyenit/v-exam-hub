# Stage 1: Build Go binary
FROM golang:1.25-alpine AS builder
WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build -o server main.go

# Stage 2: Runtime with LibreOffice for legacy DOC conversion
FROM alpine:latest
WORKDIR /root/

RUN apk add --no-cache \
    ca-certificates \
    fontconfig \
    libreoffice \
    ttf-dejavu

ENV SOFFICE_PATH=/usr/bin/soffice

COPY --from=builder /app/server ./

EXPOSE 8080
CMD ["./server"]
