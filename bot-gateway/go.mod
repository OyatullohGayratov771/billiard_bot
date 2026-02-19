module bot-gateway

go 1.25.0

require (
	github.com/go-telegram-bot-api/telegram-bot-api/v5 v5.5.1
	github.com/joho/godotenv v1.5.1
	google.golang.org/grpc v1.79.1
	google.golang.org/protobuf v1.36.11 // indirect
)

require (
	golang.org/x/net v0.48.0 // indirect
	golang.org/x/sys v0.39.0 // indirect
	golang.org/x/text v0.32.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20251202230838-ff82c1b0f217 // indirect
)

require github.com/gayratovoyatulloh/billiard-bot-proto v0.0.0

replace github.com/gayratovoyatulloh/billiard-bot-proto => ../proto

require user-service v0.0.0

replace user-service => ../user-service
