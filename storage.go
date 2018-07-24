package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"strings"
	"text/template"
	"time"

	"github.com/gocarina/gocsv"
	_ "github.com/mattn/go-sqlite3"
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
	if m.Likes == 0 {
		return 0
	}
	var repostsToLikes = math.Min(float64(m.Reposts)/float64(m.Likes), 1)
	return repostsToLikes + (1-repostsToLikes)*float64(m.Likes)/float64(m.Views)
}

func (m *Meme) calculateTimeCoeff() float64 {
	x := float64(time.Now().Sub(m.Time)) / float64(time.Hour)

	return 1 / math.Exp(x/Config.Metric.Coeff)
}

func (m *Meme) calculateGroupCoeff() float64 {
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

func (m *Meme) calculateKekScore() float64 {
	if m.Views == 0 {
		return 0
	}
	var summedWeight = kekIndexWeight + timeCoeffWeight + groupCoeffWeight + groupActivityWeight

	score := (kekIndexWeight*m.calculateKekIndex() + timeCoeffWeight*m.calculateTimeCoeff() + groupCoeffWeight*m.calculateGroupCoeff() /*+ groupActivityWeight*m.calculateGroupActivity()*/) / summedWeight //group coeff is unclear for me, need reconsideration of this coeff
	return score * 10
}

func generateURL(platform string, meme Meme) string {
	var err error
	buf := bytes.NewBuffer([]byte(""))

	switch strings.ToLower(platform) {
	case "vk":
		{
			if _, ok := Config.VK.Publics[meme.Public]; !ok {
				Log.Errorf("Cannot find info to public %s", meme.Public)
			}
			tmpl, err := template.New("tmpl").Parse(Config.VK.LinkFormat)
			if err != nil {
				Log.Errorf("Cannot parse template for %s. Reason %s", platform, err)
			}
			err = tmpl.ExecuteTemplate(buf, "tmpl", map[string]interface{}{
				"Group":   meme.Public,
				"GroupId": Config.VK.Publics[meme.Public].GroupId,
				"PostId":  meme.Id,
			})
		}
	}
	if err != nil {
		Log.Errorf("Cannot execute template %s. Reason %s", strings.ToLower(platform), err)
	}

	return buf.String()
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
	//insert into memes_new (memeid, public, platform, pictures, description, likes, reposts, views, comments, time) select * from memes;
	//insert into memes select * from memes_new;
	//insert into memes_new (memeid, public, platform, pictures, description, likes, reposts, views, comments, time) select * from memes;
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
	//insert into shown_memes_new select m.id, sm.chat_id, sm.msg_id from shown_memes as sm join memes as m on m.platform = sm.platform and m.public = sm.public and m.memeid = sm.id;
	//insert into shown_memes_new select m.id, sm.chat_id, sm.msg_id from shown_memes as sm join memes as m on sm.id=m.memeid and sm.public=m.public and sm.platform=m.platform;
	_, err = s.DB.Exec(`CREATE TABLE IF NOT EXISTS shown_memes (
meme_id INTEGER NOT NULL,
chat_id int NOT NULL,
msg_id int NOT NULL,
FOREIGN KEY(meme_id) REFERENCES memes(id)
)`)
	if err != nil {
		return fmt.Errorf("Cannot create shown_memes table. Reason %s", err)
	}
	//insert into meme_hashes_new select m.id, mh.hash from meme_hashes as mh join memes as m on m.platform = mh.platform and m.public = mh.public and m.memeid = mh.id;
	//insert into meme_hashes_new select m.id, sm.hash from meme_hashes as sm join memes as m on sm.id=m.memeid and sm.public=m.public and sm.platform=m.platform;
	_, err = s.DB.Exec(`CREATE TABLE IF NOT EXISTS meme_hashes (
meme_id INTEGER NOT NULL,
hash TEXT NOT NULL,
FOREIGN KEY(meme_id) REFERENCES memes(id)
)`)
	if err != nil {
		return fmt.Errorf("Cannot create meme_hashes table. Reason %s", err)
	}
	//insert into memes(id, memeid, public, platform, pictures, description, likes, reposts, views, comments, time) values(0, 0, "fedormemes", "fedormemes", "[]", "", 1, 1, 100000, 1, "2000-00-00 00:00:00");
	//insert into chat_metadata_new select *, 0 from chat_metadata;
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
	hash := ""

	if isExist {
		return nil
	}

	var isUnique bool
	isUnique, hash, err = s.isUnique(&meme)
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
	err = s.Init()
	if err != nil {
		return nil, err
	}

	time.AfterFunc(time.Second, func() {
		groupRatings, err := storage.CalculateGroupRating(Config.TelegramBot.ChatId)
		if err != nil {
			Log.Errorf("Cannot calculate group rating. Reason %s", err)
		}
		storage.GroupRatings = groupRatings
		Log.Infof("GroupRatings %v", storage.GroupRatings)

		groupsActivity, err := storage.CalculateGroupActivity()
		if err != nil {
			Log.Errorf("Cannot calculate group activity. Reason %s", err)
		}
		storage.GroupActivity = groupsActivity
	})
	return &s, nil

}
