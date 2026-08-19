FROM golang:1.22-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build -o live-service ./cmd

FROM alpine:3.19
# FFmpeg — RTSP → HLS jonli oqim uchun
RUN apk add --no-cache ca-certificates tzdata ffmpeg
ENV TZ=Asia/Tashkent

WORKDIR /app
COPY --from=builder /app/live-service .

RUN mkdir -p /app/hls /app/recordings
CMD ["./live-service"]
