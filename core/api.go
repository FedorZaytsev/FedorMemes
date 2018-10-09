package main

import (
  "fmt"
  "google.golang.org/grpc"

  pb "github.com/FedorZaytsev/FedorMemes"
)

type APIConnect struct {
  Name string
  Connect pb.ConsumerAPIClient
}

type ApiConfig struct {
  Name string
  Address string
}

func NewApiConnect(config ApiConfig) (*APIConnect, error) {
  api := APIConnect{}
  conn, err := grpc.Dial(config.Address, grpc.WithInsecure())
	if err != nil {
		return &api, fmt.Errorf("Cannot dial api. Reason %s", err)
	}
	api.Connect = pb.NewConsumerAPIClient(conn)

  return &api, nil
}
