package main

import (
	"encoding/json"
	"math"
	"time"

	pb "github.com/FedorZaytsev/FedorMemes"
)

type Meme struct {
	Id          int
	MemeId      string
	Public      string
	Platform    string
	Pictures    Pictures
	Description string
	Likes       int
	Reposts     int
	Views       int
	Comments    int
	Time        time.Time
}

type Pictures []string

func (p *Pictures) MarshalCSV() (string, error) {
	data, err := json.Marshal(p)
	return string(data), err
}

func calculateKekIndex(m *pb.Meme) float64 {
	if m.Views == 0 {
		return 0
	}
	return float64(m.Likes) / float64(m.Views) * float64(m.Reposts) / float64(m.Views) * 1000000
}

func calculateTimeCoeff(m *pb.Meme) float64 {
	x := float64(time.Now().Sub(time.Unix(m.Time, 0))) / float64(time.Hour)

	return 1 / math.Exp(x/Config.Metric.Coeff)
}
