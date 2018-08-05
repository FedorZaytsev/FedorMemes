package main

import (
	"encoding/json"
	"math"
	"time"
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

type MemeDebug struct {
	Meme
	KekIndex      float64
	TimePassed    string
	TimeCoeff     float64
	GroupCoeff    float64
	GroupActivity float64
	KekScore      float64
}

type Pictures []string

func (p *Pictures) MarshalCSV() (string, error) {
	data, err := json.Marshal(p)
	return string(data), err
}

func (m *Meme) calculateKekIndex() float64 {
	if m.Views == 0 {
		return 0
	}
	return float64(m.Likes) / float64(m.Views) * float64(m.Reposts) / float64(m.Views) * 1000000
}

func (m *Meme) calculateTimeCoeff() float64 {
	x := float64(time.Now().Sub(m.Time)) / float64(time.Hour)

	return 1 / math.Exp(x/Config.Metric.Coeff)
}

func (m *Meme) calculateGroupRating() float64 {
	if _, ok := storage.GroupRatings[m.Platform]; !ok {
		if def, ok := Config.Metric.DefaultGroupRating[m.Platform]; ok {
			return def
		}
		return 1.0
	}
	groupRating, ok := storage.GroupRatings[m.Platform][m.Public]
	if !ok {
		if def, ok := Config.Metric.DefaultGroupRating[m.Platform]; ok {
			return def
		}
		return 1.0
	}
	return groupRating
}

func (m *Meme) calculateGroupActivity() float64 {
	if _, ok := storage.GroupActivity[m.Platform]; !ok {
		Log.Infof("calculateGroupActivity Cannot find %s in %v", m.Platform, storage.GroupActivity)
		return 1.0
	}
	if _, ok := storage.GroupActivity[m.Platform][m.Public]; !ok {
		Log.Infof("calculateGroupActivity Cannot find %s in %v", m.Platform, storage.GroupActivity)
		return 1.0
	}
	return storage.GroupActivity[m.Platform][m.Public]
}

func (m *Meme) calculatePlatformRating() float64 {
	rating, ok := storage.PlatformRatings[m.Platform]
	if !ok {
		if def, ok := Config.Metric.DefaultGroupRating[m.Platform]; ok {
			return def
		}
		return 1.0
	}
	return rating
}

func (m *Meme) calculatePlatformActivity() float64 {
	rating, ok := storage.PlatformActivity[m.Platform]
	if !ok {
		Log.Infof("calculateGroupActivity Cannot find %s in %v", m.Platform, storage.GroupActivity)
		return 1.0
	}
	return rating
}

func (m *Meme) СalculateKekScore() float64 {
	if m.Views == 0 {
		return 0
	}
	//var summedWeight = kekIndexWeight + timeCoeffWeight + groupCoeffWeight + groupActivityWeight

	score := m.calculateKekIndex() * m.calculateTimeCoeff()
	score = score / m.calculateGroupActivity() * m.calculateGroupRating()
	score = score / m.calculatePlatformActivity() * m.calculatePlatformRating()

	//score := (kekIndexWeight*m.calculateKekIndex() + timeCoeffWeight*m.calculateTimeCoeff() + groupCoeffWeight*m.calculateGroupRating() /*+ groupActivityWeight*m.calculateGroupActivity()*/) / summedWeight //group coeff is unclear for me, need reconsideration of this coeff
	return score
}
