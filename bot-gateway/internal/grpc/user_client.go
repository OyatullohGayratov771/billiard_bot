package grpc

import (
	"context"
	"time"

	userpb "github.com/gayratovoyatulloh/billiard-bot-proto/user/v1"


	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

type UserClient struct {
	client userpb.UserServiceClient
	apiKey string
}

func NewUserClient(addr, apiKey string) (*UserClient, error) {
	conn, err := grpc.Dial(addr, grpc.WithInsecure())
	if err != nil {
		return nil, err
	}

	return &UserClient{
		client: userpb.NewUserServiceClient(conn),
		apiKey: apiKey,
	}, nil
}

func (u *UserClient) Register(
	ctx context.Context,
	telegramID int64,
	username, firstName, lastName string,
) (*userpb.User, error) {

	md := metadata.New(map[string]string{
		"x-api-key": u.apiKey,
	})
	ctx = metadata.NewOutgoingContext(ctx, md)

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	resp, err := u.client.Register(ctx, &userpb.RegisterRequest{
		TelegramId: telegramID,
		Username:   username,
		FirstName:  firstName,
		LastName:   lastName,
	})
	if err != nil {
		return nil, err
	}

	return resp.User, nil
}
