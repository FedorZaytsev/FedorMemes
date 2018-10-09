package main

import (
	"fmt"
	"math"
	"context"
	"time"

	pb "github.com/FedorZaytsev/FedorMemes"
)

func (s *Storage) calculateGroupActivity(memes []*pb.Meme) error {
	res := map[string]map[string]float64{}
	temp := map[string]map[string][]float64{}

	for _, memI := range memes {
		meme := Meme{
			Meme: memI,
		}
		if _, ok := temp[meme.Platform]; !ok {
			temp[meme.Platform] = make(map[string][]float64)
		}
		temp[meme.Platform][meme.Public] = append(temp[meme.Platform][meme.Public], meme.calculateKekIndex())
	}

	for platform := range temp {
		if _, ok := res[platform]; !ok {
			res[platform] = make(map[string]float64)
		}
		for public := range temp[platform] {
			sum := float64(0)
			for _, el := range temp[platform][public] {
				sum += el
			}
			res[platform][public] = sum / float64(len(temp[platform][public]))
		}
	}

	s.GroupActivity = res

	return nil
}

func (s *Storage) calculatePlatformActivity(memes []*pb.Meme) error {
	res := map[string]float64{}
	temp := map[string][]float64{}

	for _, memI := range memes {
		meme := Meme{
			Meme: memI,
		}
		score := meme.calculateKekIndex() / meme.calculateGroupActivity() * meme.calculateGroupRating()
		temp[meme.Platform] = append(temp[meme.Platform], score)
	}

	for platform := range temp {
		sum := float64(0)
		for _, el := range temp[platform] {
			sum += el
		}
		res[platform] = sum / float64(len(temp[platform]))
	}

	s.PlatformActivity = res

	return nil
}

func (s *Storage) calculateGroupRating(chatId int64) error {
	type Counters struct {
		Likes    float64
		Dislikes float64
	}

	rating := map[string]map[string]float64{}
	counters := map[string]map[string]Counters{}
	stats, err := s.getStatistics(chatId)
	if err != nil {
		return fmt.Errorf("Cannot get statistics. Reason %s", err)
	}

	for _, stat := range stats {
		if _, ok := counters[stat.Platform]; !ok {
			counters[stat.Platform] = make(map[string]Counters)
		}
		counter := counters[stat.Platform][stat.Public]
		counter.Likes = counter.Likes + float64(stat.Likes)
		counter.Dislikes = counter.Dislikes + float64(stat.Dislikes)
		counters[stat.Platform][stat.Public] = counter
	}

	for platform := range counters {
		for public := range counters[platform] {
			if _, ok := rating[platform]; !ok {
				rating[platform] = make(map[string]float64)
			}
			counters := counters[platform][public]
			likes := counters.Likes
			dislikes := counters.Dislikes
			if likes+dislikes == 0 {
				rating[platform][public] = 0.5
			} else {
				rating[platform][public] = -1.0/(math.Exp((likes-dislikes)/(likes+dislikes))+1) + 1
			}
		}
	}

	s.GroupRatings = rating

	return nil
}

func (s *Storage) calculatePlatformRating(chatId int64) error {
	type Counters struct {
		Likes    float64
		Dislikes float64
	}

	rating := map[string]float64{}
	counters := map[string]Counters{}
	stats, err := s.getStatistics(chatId)
	if err != nil {
		return fmt.Errorf("Cannot get statistics. Reason %s", err)
	}

	for _, stat := range stats {
		counter := counters[stat.Platform]
		counter.Likes += float64(stat.Likes)
		counter.Dislikes += float64(stat.Dislikes)
		counters[stat.Platform] = counter
	}

	for platform := range counters {
		counters := counters[platform]
		likes := counters.Likes
		dislikes := counters.Dislikes
		if likes+dislikes == 0 {
			rating[platform] = 0.5
		} else {
			rating[platform] = -1.0/(math.Exp((likes-dislikes)/(likes+dislikes))+1) + 1
		}
	}

	s.PlatformRatings = rating

	return nil
}

func (s *Storage) calculateCoeffs(chatId int64, memes []*pb.Meme) error {
	var err error
	err = s.calculateGroupRating(chatId)
	if err != nil {
		return fmt.Errorf("Cannot calculate group rating. Reason %s", err)
	}

	err = s.calculateGroupActivity(memes)
	if err != nil {
		return fmt.Errorf("Cannot calculate group activity. Reason %s", err)
	}

	err = s.calculatePlatformRating(chatId)
	if err != nil {
		return fmt.Errorf("Cannot calculate platform rating. Reason %s", err)
	}

	err = s.calculatePlatformActivity(memes)
	if err != nil {
		return fmt.Errorf("Cannot calculate platform activity. Reason %s", err)
	}

	return nil

}

type MemeStat struct {
	MemeId     string
	Public     string
	Platform   string
	Pictures   string
	Likes      int32
	Dislikes   int32
	KekIndex   float64
	TimeCoeff  float64
	GroupCoeff float64
}

func (s *Storage) getStatistics(chatId int64) ([]MemeStat, error) {
	res := []MemeStat{}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second * time.Duration(s.Timeout))
	defer cancel()

	info, err := s.Connect.GetShownMemesInfo(ctx, &pb.GetShownMemesInfoRequest{
		ChatId: chatId,
	})
	if err != nil {
		return res, fmt.Errorf("Cannot get shown memes. Reason %s", err)
	}


	for _, smeme := range info.Response {
		memI, err := s.GetMemeById(smeme.MemeId)
		if err != nil {
			return res, fmt.Errorf("Cannot get meme %d. Reason %s", smeme.MemeId, err)
		}

		meme := Meme{
			Meme: memI,
		}


		res = append(res, MemeStat{
			MemeId:     meme.MemeId,
			Public:     meme.Public,
			Platform:   meme.Platform,
			Pictures:   fmt.Sprintf("%v", meme.Pictures),
			Likes:      smeme.Likes,
			Dislikes:   smeme.Dislikes,
			KekIndex:   meme.calculateKekIndex(),
			TimeCoeff:  meme.calculateTimeCoeff(),
			GroupCoeff: meme.calculateGroupRating(),
		})
	}

	return res, nil
}
