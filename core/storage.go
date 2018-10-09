package main

import (
  "fmt"
  "time"
  "context"

  "google.golang.org/grpc"

  pb "github.com/FedorZaytsev/FedorMemes"
)

var storage *Storage

type StorageConfig struct {
  Address string
  Timeout int
  TimeoutCache int
}

type Storage struct {
  cacheTime time.Time
  cacheMemes []*pb.Meme
  Timeout time.Duration
  TimeoutCache time.Duration
  GroupRatings     map[string]map[string]float64
	GroupActivity    map[string]map[string]float64
	PlatformRatings  map[string]float64
	PlatformActivity map[string]float64
  Connect pb.StorageClient
}

func (s *Storage) getAllMemes() []*pb.Meme {
  if s.cacheTime.After(time.Now()) {
    return s.cacheMemes
  } else {
    ctx, cancel := context.WithTimeout(context.Background(), time.Second * time.Duration(s.Timeout))
    defer cancel()

    memes, err := s.Connect.GetMemes(ctx, &pb.GetMemesStorageRequest{
      From: 0,
    })
    if err != nil {
      Log.Errorf("Cannot get memes. Reason %s", err)
    }
    s.cacheMemes = memes.Response

    s.cacheTime = time.Now().Add(s.TimeoutCache)

    return s.cacheMemes
  }
}

func (s *Storage) GetUnshownMemes(chatId int64, from time.Time) ([]*pb.Meme, error) {
  ctx, cancel := context.WithTimeout(context.Background(), time.Second * time.Duration(s.Timeout))
  defer cancel()

  memes, err := s.Connect.GetMemesUnshown(ctx, &pb.GetMemesStorageUnshownRequest{
    From: from.Unix(),
    ChatId: chatId,
  })
  if err != nil {
    return memes.Response, fmt.Errorf("Cannot get unshown memes. Reason %s", err)
  }
  err = s.calculateCoeffs(chatId, s.getAllMemes())
  if err != nil {
    return memes.Response, fmt.Errorf("Cannot update coeffs. Reason %s", err)
  }

  return memes.Response, nil
}

func (s *Storage) MarkMemeShown(chatId int64, msgId int32, memeid int32) error {
  ctx, cancel := context.WithTimeout(context.Background(), time.Second * time.Duration(s.Timeout))
  defer cancel()

  _, err := s.Connect.MarkMemeShown(ctx, &pb.MarkMemeShownRequest{
    ChatId: chatId,
    MsgId: msgId,
    MemeId: memeid,
  })
  return err
}

func (s *Storage) GetMemes(from time.Time) ([]*pb.Meme, error) {
  ctx, cancel := context.WithTimeout(context.Background(), time.Second * time.Duration(s.Timeout))
  defer cancel()

  memes, err := s.Connect.GetMemes(ctx, &pb.GetMemesStorageRequest{
    From: from.Unix(),
  })
  if err != nil {
    return memes.Response, fmt.Errorf("Cannot get unshown memes. Reason %s", err)
  }
  return memes.Response, nil
}

func (s *Storage) GetMemeById(id int32) (*pb.Meme, error) {
  ctx, cancel := context.WithTimeout(context.Background(), time.Second * time.Duration(s.Timeout))
  defer cancel()

  resp, err := s.Connect.GetMemeById(ctx, &pb.GetMemeByIdRequest{
    Id: id,
  })
  if err != nil {
    return resp.Meme, fmt.Errorf("Cannot get meme %d. Reason %s", id, err)
  }
  return resp.Meme, nil
}

func (s *Storage) MakeAction(platform string, chatId int64, messageId, userId, btnId int) error {
  ctx, cancel := context.WithTimeout(context.Background(), time.Second * time.Duration(s.Timeout))
  defer cancel()

  _, err := s.Connect.MakeAction(ctx, &pb.MakeActionRequest{
    Platform: platform,
    ChatId: chatId,
    MessageId: int32(messageId),
    UserId: int32(userId),
    BtnId: int32(btnId),
  })
  if err != nil {
    return fmt.Errorf("Cannot make action. Reason %s", err)
  }
  return nil
}

func (s *Storage) CalculateCounter(platform string, chatId int64, messageId int) (map[int32]int32, error) {
  ctx, cancel := context.WithTimeout(context.Background(), time.Second * time.Duration(s.Timeout))
  defer cancel()

  resp, err := s.Connect.GetMemeInfo(ctx, &pb.GetMemeInfoRequest{
    Platform: platform,
    ChatId: chatId,
    MessageId: int32(messageId),
  })
  if err != nil {
    return resp.Reactions, fmt.Errorf("Cannot make action. Reason %s", err)
  }
  return resp.Reactions, nil
}

func NewStorage(config StorageConfig) (*Storage, error) {
  st := Storage{
    GroupRatings: make(map[string]map[string]float64),
    GroupActivity: make(map[string]map[string]float64),
    PlatformRatings: make(map[string]float64),
    PlatformActivity: make(map[string]float64),
  }
  conn, err := grpc.Dial(config.Address, grpc.WithInsecure())
  if err != nil {
    return &st, fmt.Errorf("Cannot dial storage. Reason %s", err)
  }
  st.Connect = pb.NewStorageClient(conn)

  return &st, nil
}
