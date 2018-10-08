package main

import (
	"context"
	"fmt"
	"time"

	pb "github.com/FedorZaytsev/FedorMemes"
	"github.com/golang/protobuf/ptypes/empty"
)

type Server struct {
}

func (s Server) GetMemes(ctx context.Context, _ *empty.Empty) (*pb.GetMemesResponse, error) {
	until := time.Now().Add(-time.Duration(Config.VK.LookingDuration) * time.Hour)
	memes, err := Config.VK.updateMemes(until)
	if err != nil {
		return nil, fmt.Errorf("Cannot get memes. Reason %s", err)
	}
	return &pb.GetMemesResponse{
		Response: memes,
	}, nil
}

func (s Server) GetMemesFrom(ctx context.Context, in *pb.GetMemesRequest) (*pb.GetMemesResponse, error) {
	until := time.Unix(in.From, 0)
	memes, err := Config.VK.updateMemes(until)
	if err != nil {
		return nil, fmt.Errorf("Cannot get memes. Reason %s", err)
	}
	return &pb.GetMemesResponse{
		Response: memes,
	}, nil
}
