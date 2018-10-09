package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	_ "github.com/mattn/go-sqlite3"

	pb "github.com/FedorZaytsev/FedorMemes"
)

var storage *Storage

var kekIndexWeight = 2.0
var timeCoeffWeight = 1.0
var groupCoeffWeight = 1.0
var groupActivityWeight = 1.0

type Storage struct {
	DB *sql.DB
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

	return nil
}

func (s *Storage) isMemeExists(id, public, platform string) (bool, error) {
	exist := 0
	err := s.DB.QueryRow("SELECT EXISTS(SELECT 1 FROM memes WHERE platform = ? and public = ? and memeid = ?)", platform, public, id).Scan(&exist)
	return exist == 1, err
}

func (s *Storage) GetMemeById(id int) (*pb.Meme, error) {
	m := pb.Meme{}
	rows, err := s.DB.Query("SELECT * FROM memes WHERE id = ?", id)
	if err != nil {
		return &m, fmt.Errorf("Cannot get meme from db. Reason %s", err)
	}
	memes, err := s.parseGetMemesAnswer(rows)
	if err != nil {
		return &m, err
	}
	if len(memes) > 0 {
		return memes[0], nil
	}
	return nil, NotFound
}

func (s *Storage) AddMeme(meme *pb.Meme) error {
	isExist, err := s.isMemeExists(meme.MemeId, meme.Public, meme.Platform)
	if err != nil {
		return fmt.Errorf("Cannot check is meme exist. Reason %s", err)
	}

	if isExist {
		return nil
	}

	isUnique, hash, err := s.isUnique(meme)
	if err != nil {
		return fmt.Errorf("Cannot check is meme %v unique. Reason %s", meme, err)
	}

	if !isUnique {
		return nil
	}

	//Log.Infof("New meme %v", meme)

	pictures, err := json.Marshal(meme.Pictures)
	if err != nil {
		return fmt.Errorf("Cannot marshal meme.Pictures. Reason %s", err)
	}

	res, err := s.DB.Exec("INSERT OR REPLACE INTO memes (memeid, public, platform, pictures, description, likes, reposts, views, comments, time) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		meme.MemeId, meme.Public, meme.Platform, pictures, meme.Description, meme.Likes, meme.Reposts, meme.Views, meme.Comments, time.Unix(meme.Time, 0).Format(ISO8601))
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

func (s *Storage) parseGetMemesAnswer(rows *sql.Rows) ([]*pb.Meme, error) {
	res := []*pb.Meme{}
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

		res = append(res, &pb.Meme{
			Id:          int32(id),
			MemeId:      memeid,
			Public:      public,
			Platform:    platform,
			Pictures:    pictures,
			Description: description,
			Likes:       int32(likes),
			Reposts:     int32(reposts),
			Views:       int32(views),
			Comments:    int32(comments),
			Time:        t.Unix(),
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

func (s *Storage) GetMemes(from time.Time) ([]*pb.Meme, error) {
	rows, err := s.DB.Query("SELECT * FROM memes WHERE time > ?", from.Format(ISO8601))
	if err != nil {
		return []*pb.Meme{}, fmt.Errorf("Cannot get memes. Reason %s", err)
	}
	defer rows.Close()

	return s.parseGetMemesAnswer(rows)
}

func (s *Storage) GetShownMemesInfo(chatId int64) ([]*pb.ShownMemeInfo, error) {
	shownmemes := []*pb.ShownMemeInfo{}

	rows, err := s.DB.Query(`select meme_id, msg_id from shown_memes where msg_id != 0 and chat_id = ?`, chatId)
	if err != nil {
		return shownmemes, fmt.Errorf("Cannot get shown memes. Reason %s", err)
	}
	defer rows.Close()

	for rows.Next() {
		var info pb.ShownMemeInfo
		err = rows.Scan(&info.MemeId, &info.MsgId)
		if err != nil {
			return shownmemes, fmt.Errorf("Cannot scan values from db. Reason %s", err)
		}

		err = s.DB.QueryRow(`select t1.likes, t2.dislikes from
			(select count(*) as likes from chat_metadata where chat_id=? and msg_id = ? and btn_id=0) as t1
			join
			(select count(*) as dislikes from chat_metadata where chat_id=? and msg_id = ? and btn_id=1) as t2;`, chatId, info.MsgId, chatId, info.MsgId).Scan(&info.Likes, &info.Dislikes)
		if err != nil {
			return shownmemes, fmt.Errorf("Cannot get counter for message %d. Reason %s", info.MsgId, err)
		}

		shownmemes = append(shownmemes, &info)
	}
	return shownmemes, nil
}

func (s *Storage) GetUnshownMemes(chatId int64, from time.Time) ([]*pb.Meme, error) {
	rows, err := s.DB.Query(`select * from memes as m where time > ? and EXISTS(
	select 1 from shown_memes as sm where m.id == sm.meme_id and sm.chat_id == ?
) == 0`, from.Format(ISO8601), chatId)
	if err != nil {
		return []*pb.Meme{}, fmt.Errorf("Cannot get memes. Reason %s", err)
	}
	defer rows.Close()

	return s.parseGetMemesAnswer(rows)
}

func NewStorage(dbname string) (*Storage, error) {
	db, err := sql.Open("sqlite3", dbname)
	if err != nil {
		return nil, fmt.Errorf("Cannot open sqlite. Reason %s", err)
	}

	s := Storage{
		DB: db,
	}

	return &s, nil

}
