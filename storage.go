package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"time"

	"github.com/gocarina/gocsv"
	_ "github.com/mattn/go-sqlite3"
	"runtime/debug"
)

var storage *Storage

var kekIndexWeight = 2.0
var timeCoeffWeight = 1.0
var groupCoeffWeight = 1.0
var groupActivityWeight = 1.0

type Storage struct {
	DB            *sql.DB
	GroupRatings  map[string]map[string]float64
	GroupActivity map[string]map[string]float64
}

func (s *Storage) CalculateGroupActivity() (map[string]map[string]float64, error) {
	res := map[string]map[string]float64{}
	temp := map[string]map[string][]float64{}
	memes, err := s.GetMemes(time.Unix(0, 0))
	if err != nil {
		return res, fmt.Errorf("Cannot get memes. Reason %s", err)
	}

	for _, meme := range memes {
		if _, ok := temp[meme.Platform]; !ok {
			temp[meme.Platform] = make(map[string][]float64)
		}
		if _, ok := temp[meme.Platform][meme.Public]; !ok {
			temp[meme.Platform][meme.Public] = []float64{}
		}
		temp[meme.Platform][meme.Public] = append(temp[meme.Platform][meme.Public], meme.calculateKekIndex())
	}

	for platform, _ := range temp {
		if _, ok := res[platform]; !ok {
			res[platform] = make(map[string]float64)
		}
		for public, _ := range temp[platform] {
			sum := float64(0)
			for _, el := range temp[platform][public] {
				sum += el
			}
			res[platform][public] = sum / float64(len(temp[platform][public]))
		}
	}

	Log.Infof("CalculateGroupActivity res %v\n\n", res)

	return res, nil
}

func (s *Storage) CalculateGroupRating(chatId int64) (map[string]map[string]float64, error) {
	type Counters struct {
		Likes    float64
		Dislikes float64
	}

	rating := map[string]map[string]float64{}
	counters := map[string]map[string]Counters{}
	Log.Infof("CalculateGroupRating %p", s)
	debug.PrintStack()
	stats, err := s.getStatistics(chatId)
	if err != nil {
		return rating, fmt.Errorf("Cannot get statistics. Reason %s", err)
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

	for platform, _ := range counters {
		for public, _ := range counters[platform] {
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

	return rating, nil
}

const ISO8601 = "2006-01-02 15:04:05"

var NotFound = fmt.Errorf("Doesn't exist")

func (s *Storage) Init() error {
	var err error
	_, err = s.DB.Exec(`PRAGMA foreign_keys = ON`)
	if err != nil {
		return fmt.Errorf("Cannot set pragma foreign_keys on. Reason %s", err)
	}

	_, err = s.DB.Exec(`CREATE TABLE IF NOT EXISTS memes (
id INTEGER PRIMARY KEY AUTOINCREMENT,
memeid TEXT NOT NULL,
public TEXT NOT NULL,
platform TEXT NOT NULL,
pictures TEXT NOT NULL,
description TEXT,
likes INTEGER,
reposts INTEGER,
views INTEGER,
comments INTEGER,
time TEXT NOT NULL,
UNIQUE (memeid, public, platform)
)`)
	if err != nil {
		return fmt.Errorf("Cannot create memes table. Reason %s", err)
	}

	_, err = s.DB.Exec(`CREATE TABLE IF NOT EXISTS shown_memes (
meme_id INTEGER NOT NULL,
chat_id int NOT NULL,
msg_id int NOT NULL,
FOREIGN KEY(meme_id) REFERENCES memes(id)
)`)
	if err != nil {
		return fmt.Errorf("Cannot create shown_memes table. Reason %s", err)
	}

	_, err = s.DB.Exec(`CREATE TABLE IF NOT EXISTS meme_hashes (
meme_id INTEGER NOT NULL,
hash TEXT NOT NULL,
FOREIGN KEY(meme_id) REFERENCES memes(id)
)`)
	if err != nil {
		return fmt.Errorf("Cannot create meme_hashes table. Reason %s", err)
	}

	_, err = s.DB.Exec(`CREATE TABLE IF NOT EXISTS chat_metadata (
msg_id INTEGER NOT NULL,
user_id INTEGER NOT NULL,
btn_id INTEGER NOT NULL,
chat_id INTEGER NOT NULL,
UNIQUE (msg_id, user_id, chat_id)
)`)
	if err != nil {
		return fmt.Errorf("Cannot create chat table. Reason %s", err)
	}

	s.GroupRatings, err = s.CalculateGroupRating(Config.TelegramBot.ChatId)
	if err != nil {
		Log.Errorf("Cannot calculate group rating. Reason %s", err)
	}
	Log.Infof("GroupRatings %v", s.GroupRatings)

	s.GroupActivity, err = s.CalculateGroupActivity()
	if err != nil {
		Log.Errorf("Cannot calculate group activity. Reason %s", err)
	}

	return nil
}

func (s *Storage) isMemeExists(id, public, platform string) (bool, error) {
	exist := 0
	err := s.DB.QueryRow("SELECT EXISTS(SELECT 1 FROM memes WHERE platform = ? and public = ? and memeid = ?)", platform, public, id).Scan(&exist)
	return exist == 1, err
}

func (s *Storage) GetMemeById(id int) (*Meme, error) {
	m := Meme{}
	rows, err := s.DB.Query("SELECT * FROM memes WHERE id = ?", id)
	if err != nil {
		return &m, fmt.Errorf("Cannot get meme from db. Reason %s", err)
	}
	memes, err := s.parseGetMemesAnswer(rows)
	if err != nil {
		return &m, err
	}
	if len(memes) > 0 {
		return &memes[0], nil
	}
	return nil, NotFound
}

func (s *Storage) AddMeme(meme Meme) error {
	isExist, err := s.isMemeExists(meme.MemeId, meme.Public, meme.Platform)
	if err != nil {
		return fmt.Errorf("Cannot check is meme exist. Reason %s", err)
	}

	if isExist {
		return nil
	}

	isUnique, hash, err := s.isUnique(&meme)
	if err != nil {
		return fmt.Errorf("Cannot check is meme %v unique. Reason %s", meme, err)
	}

	if !isUnique {
		return nil
	}

	Log.Infof("New meme %v", meme)

	pictures, err := json.Marshal(meme.Pictures)
	if err != nil {
		return fmt.Errorf("Cannot marshal meme.Pictures. Reason %s", err)
	}

	res, err := s.DB.Exec("INSERT OR REPLACE INTO memes (memeid, public, platform, pictures, description, likes, reposts, views, comments, time) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		meme.MemeId, meme.Public, meme.Platform, pictures, meme.Description, meme.Likes, meme.Reposts, meme.Views, meme.Comments, meme.Time.Format(ISO8601))
	if err != nil {
		return fmt.Errorf("Cannot insert meme %v. Reason %s", meme, err)
	}

	id, err := res.LastInsertId()
	if err != nil {
		return fmt.Errorf("Cannot get last insert id. Reason %s", err)
	}

	_, err = s.DB.Exec("INSERT OR REPLACE INTO meme_hashes (meme_id, hash) VALUES(?, ?)", id, hash)
	if err != nil {
		return fmt.Errorf("Cannot add hash to mem_hashes table. Reason %s", err)
	}

	return nil
}

func (s *Storage) parseGetMemesAnswer(rows *sql.Rows) ([]Meme, error) {
	res := []Meme{}
	var err error
	for rows.Next() {
		var (
			id                                                 int
			memeid, public, platform, picturesStr, description string
			likes, reposts, views, comments                    int
			timeStr                                            string
		)
		err = rows.Scan(&id, &memeid, &public, &platform, &picturesStr, &description, &likes, &reposts, &views, &comments, &timeStr)
		if err != nil {
			return res, fmt.Errorf("Cannot scan from row. Reason %s", err)
		}

		pictures := []string{}
		err = json.Unmarshal([]byte(picturesStr), &pictures)
		if err != nil {
			return res, fmt.Errorf("Cannot unmarshal pictures for meme %d. Reason %s", id, err)
		}
		t, err := time.Parse(ISO8601, timeStr)
		if err != nil {
			return res, fmt.Errorf("Cannot parse time for meme %d. Reason %s", id, err)
		}

		res = append(res, Meme{
			Id:          id,
			MemeId:      memeid,
			Public:      public,
			Platform:    platform,
			Pictures:    pictures,
			Description: description,
			Likes:       likes,
			Reposts:     reposts,
			Views:       views,
			Comments:    comments,
			Time:        t,
		})
	}
	return res, nil

}

func (s *Storage) MarkMemeShown(chatId int64, msgId int, memeid int) error {
	_, err := s.DB.Exec("INSERT OR REPLACE INTO shown_memes (meme_id, chat_id, msg_id) VALUES (?, ?, ?)", memeid, chatId, msgId)
	if err != nil {
		return fmt.Errorf("Cannot mark meme as shown. Reason %s", err)
	}
	return nil
}

func (s *Storage) GetMemes(from time.Time) ([]Meme, error) {
	rows, err := s.DB.Query("SELECT * FROM memes WHERE time > ?", from.Format(ISO8601))
	if err != nil {
		return []Meme{}, fmt.Errorf("Cannot get memes. Reason %s", err)
	}
	defer rows.Close()

	return s.parseGetMemesAnswer(rows)
}

func (s *Storage) GetUnshownMemes(chatId int64, from time.Time) ([]Meme, error) {
	rows, err := s.DB.Query(`select * from memes as m where time > ? and EXISTS(
	select 1 from shown_memes as sm where m.id == sm.meme_id and sm.chat_id == ?
) == 0`, from.Format(ISO8601), chatId)
	if err != nil {
		return []Meme{}, fmt.Errorf("Cannot get memes. Reason %s", err)
	}
	defer rows.Close()

	return s.parseGetMemesAnswer(rows)
}

func (s *Storage) Dump() error {
	Log.Infof("Dumping storage...")
	memes, err := s.GetMemes(time.Unix(0, 0))
	if err != nil {
		return err
	}

	type MemeEx struct {
		Meme
		KekIndex      float64
		TimeCoeff     float64
		GroupCoeff    float64
		GroupActivity float64
		KekScore      float64
	}

	res := []MemeEx{}
	for _, meme := range memes {
		res = append(res, MemeEx{
			Meme:          meme,
			KekIndex:      meme.calculateKekIndex(),
			TimeCoeff:     meme.calculateTimeCoeff(),
			GroupCoeff:    meme.calculateGroupCoeff(),
			GroupActivity: meme.calculateGroupActivity(),
			KekScore:      meme.calculateKekScore(),
		})
	}

	f, err := os.OpenFile("./dump.csv", os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("Cannot open dump.csv. Reason %s", err)
	}
	defer f.Close()

	f.Truncate(0)
	f.Seek(0, 0)

	csvContent, err := gocsv.MarshalString(&res)
	if err != nil {
		return fmt.Errorf("Cannot marshal memes csv. Reason %s", err)
	}

	_, err = f.Write([]byte(csvContent))
	if err != nil {
		return fmt.Errorf("Cannot write memes csv. Reason %s", err)
	}

	return nil
}

func NewStorage(dbname string) (*Storage, error) {
	db, err := sql.Open("sqlite3", dbname)
	if err != nil {
		return nil, fmt.Errorf("Cannot open sqlite. Reason %s", err)
	}

	s := Storage{
		DB:           db,
		GroupRatings: nil,
	}

	return &s, nil

}
