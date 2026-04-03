FROM golang:1.22-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build -o clip-service ./cmd

FROM alpine:3.19
# FFmpeg — RTSP kamera yozish uchun
RUN apk add --no-cache ca-certificates tzdata ffmpeg
ENV TZ=Asia/Tashkent

WORKDIR /app
COPY --from=builder /app/clip-service .

# Kliplar saqlash joyi
RUN mkdir -p /app/clips
VOLUME ["/app/clips"]

CMD ["./clip-service"]
