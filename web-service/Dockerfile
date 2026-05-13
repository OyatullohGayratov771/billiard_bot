FROM golang:1.22-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build -o web-service ./cmd

FROM alpine:3.19
RUN apk add --no-cache ca-certificates tzdata
ENV TZ=Asia/Tashkent

WORKDIR /app
COPY --from=builder /app/web-service .

CMD ["./web-service"]
