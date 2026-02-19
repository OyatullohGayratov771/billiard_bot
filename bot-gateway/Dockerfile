FROM golang:1.22-alpine AS build
WORKDIR /app
COPY . .
RUN go build -o bot-gateway ./cmd

FROM alpine
WORKDIR /app
COPY --from=build /app/bot-gateway .
CMD ["./bot-gateway"]
