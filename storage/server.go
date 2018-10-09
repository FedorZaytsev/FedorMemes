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

func (s Server) AddMeme(ctx context.Context, in *pb.AddMemeRequest) (*empty.Empty, error) {
	err := storage.AddMeme(in.Meme)
	if err != nil {
		return &empty.Empty{}, fmt.Errorf("Cannot add meme. Reason %s", err)
	}
	return &empty.Empty{}, nil
}

func (s Server) MakeAction(ctx context.Context, in *pb.MakeActionRequest) (*empty.Empty, error) {
	err := storage.MakeAction(in.Platform, in.ChatId, int(in.MessageId), int(in.UserId), int(in.BtnId))
	if err != nil {
		return &empty.Empty{}, fmt.Errorf("Cannot make action. Reason %s", err)
	}
	return &empty.Empty{}, nil
}

func (s Server) MarkMemeShown(ctx context.Context, in *pb.MarkMemeShownRequest) (*empty.Empty, error) {
	err := storage.MarkMemeShown(in.ChatId, int(in.MsgId), int(in.MemeId))
	if err != nil {
		return &empty.Empty{}, fmt.Errorf("Cannot mark meme shown. Reason %s", err)
	}
	return &empty.Empty{}, nil
}

func (s Server) IsMemeExists(ctx context.Context, in *pb.IsMemeExistsRequest) (*pb.IsMemeExistsResponse, error) {
	flag, err := storage.isMemeExists(in.Id, in.Public, in.Platfrom)
	if err != nil {
		return &pb.IsMemeExistsResponse{}, fmt.Errorf("Cannot check is meme exists. Reason %s", err)
	}
	return &pb.IsMemeExistsResponse{
		Exists: flag,
	}, nil
}

func (s Server) GetMemeById(ctx context.Context, in *pb.GetMemeByIdRequest) (*pb.GetMemeByIdResponse, error) {
	meme, err := storage.GetMemeById(int(in.Id))
	if err != nil {
		return &pb.GetMemeByIdResponse{}, fmt.Errorf("Cannot get meme by id. Reason %s", err)
	}
	return &pb.GetMemeByIdResponse{
		Meme: meme,
	}, nil
}

func (s Server) GetMemes(ctx context.Context, in *pb.GetMemesStorageRequest) (*pb.GetMemesResponse, error) {
	memes, err := storage.GetMemes(time.Unix(in.From, 0))
	if err != nil {
		return &pb.GetMemesResponse{}, fmt.Errorf("Cannot get all memes. Reason %s", err)
	}
	return &pb.GetMemesResponse{
		Response: memes,
	}, nil
}

func (s Server) GetMemesUnshown(ctx context.Context, in *pb.GetMemesStorageUnshownRequest) (*pb.GetMemesResponse, error) {
	memes, err := storage.GetUnshownMemes(in.ChatId, time.Unix(in.From, 0))
	if err != nil {
		return &pb.GetMemesResponse{}, fmt.Errorf("Cannot get all unshown memes. Reason %s", err)
	}
	return &pb.GetMemesResponse{
		Response: memes,
	}, nil
}

func (s Server) GetShownMemesInfo(ctx context.Context, in *pb.GetShownMemesInfoRequest) (*pb.GetShownMemesInfoResponse, error) {
	info, err := storage.GetShownMemesInfo(in.ChatId)
	if err != nil {
		return &pb.GetShownMemesInfoResponse{}, fmt.Errorf("Cannot get shown memes info. Reason %s", err)
	}
	return &pb.GetShownMemesInfoResponse{
		Response: info,
	}, nil
}

func (s Server) GetMemeInfo(ctx context.Context, in *pb.GetMemeInfoRequest) (*pb.GetMemeInfoResponse, error) {
	info, err := storage.CalculateCounter(in.Platform, in.ChatId, int(in.MessageId))
	if err != nil {
		return &pb.GetMemeInfoResponse{}, fmt.Errorf("Cannot get meme info. Reason %s", err)
	}
	return &pb.GetMemeInfoResponse{
		Reactions: info,
	}, nil
}
