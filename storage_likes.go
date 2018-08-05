package main

import (
	"database/sql"
	"fmt"
)

func (s *Storage) MakeAction(platform string, chatId int64, messageId, userId, btnId int) error {
	var userBtnId int

	err := s.DB.QueryRow("SELECT btn_id FROM chat_metadata WHERE chat_id = ? and msg_id = ? and user_id = ?",
		chatId, messageId, userId).Scan(&userBtnId)
	if err != nil && err != sql.ErrNoRows {
		return fmt.Errorf("Cannot select metadata for msg %d. Reason %s", messageId, err)
	}

	//if not found
	if err == sql.ErrNoRows {
		_, err := s.DB.Exec("INSERT INTO chat_metadata (msg_id, user_id, btn_id, chat_id) VALUES (?, ?, ?, ?)",
			messageId, userId, btnId, chatId)

		if err != nil {
			return fmt.Errorf("Cannot insert metada. Reason %s", err)
		}
	} else {
		if userBtnId != btnId {
			_, err = s.DB.Exec("UPDATE chat_metadata SET btn_id = ? WHERE msg_id = ? and user_id = ? and chat_id = ?",
				btnId, messageId, userId, chatId)
			if err != nil {
				return fmt.Errorf("Cannot update btnId. Reason %s", err)
			}
		} else {
			_, err = s.DB.Exec("DELETE FROM chat_metadata WHERE msg_id = ? and user_id = ? and chat_id = ?",
				messageId, userId, chatId)
			if err != nil {
				return fmt.Errorf("Cannot delete btnId. Reason %s", err)
			}
		}
	}
	return nil
}

func (s *Storage) CalculateCounter(platform string, chatId int64, messageId int) (map[int]int, error) {
	btnCounters := make(map[int]int)
	rows, err := s.DB.Query("SELECT btn_id, count(*) FROM chat_metadata WHERE msg_id = ? and chat_id = ? GROUP BY btn_id", messageId, chatId)
	if err != nil {
		return btnCounters, fmt.Errorf("Cannot get counter for message %d. Reason %s", messageId, err)
	}
	defer rows.Close()
	for rows.Next() {
		var btnId int
		var count int
		err = rows.Scan(&btnId, &count)
		if err != nil {
			return btnCounters, fmt.Errorf("Cannot scan from row. Reason %s", err)
		}
		btnCounters[btnId] = count
	}

	return btnCounters, nil
}

type MemeStat struct {
	MemeId     string
	Public     string
	Platform   string
	Pictures   string
	Likes      int
	Dislikes   int
	KekIndex   float64
	TimeCoeff  float64
	GroupCoeff float64
}

func (s *Storage) getStatistics(chatId int64) ([]MemeStat, error) {
	res := []MemeStat{}
	Log.Infof("s %p", s)
	Log.Infof("s %p s.DB %p", s, s.DB)
	rows, err := s.DB.Query(`select meme_id, msg_id from shown_memes where msg_id != 0 and chat_id = ?`, chatId)
	if err != nil {
		return res, fmt.Errorf("Cannot get shown_memes for chat %d. Reason %s", chatId, err)
	}

	type ShownMeme struct {
		MemeId int
		MsgId  int
	}

	shownmemes := []ShownMeme{}

	for rows.Next() {
		var (
			memeid, msgid int
		)
		err = rows.Scan(&memeid, &msgid)
		if err != nil {
			return res, fmt.Errorf("Cannot scan values from db. Reason %s", err)
		}
		shownmemes = append(shownmemes, ShownMeme{
			MemeId: memeid,
			MsgId:  msgid,
		})
	}
	rows.Close()

	for _, smeme := range shownmemes {
		var likes, dislikes int
		err := s.DB.QueryRow(`select t1.likes, t2.dislikes from
			(select count(*) as likes from chat_metadata where chat_id=? and msg_id = ? and btn_id=0) as t1
			join
			(select count(*) as dislikes from chat_metadata where chat_id=? and msg_id = ? and btn_id=1) as t2;`, chatId, smeme.MsgId, chatId, smeme.MsgId).Scan(&likes, &dislikes)
		if err != nil {
			return res, fmt.Errorf("Cannot get counter for message %d. Reason %s", smeme.MsgId, err)
		}

		meme, err := s.GetMemeById(smeme.MemeId)
		if err != nil {
			return res, fmt.Errorf("Cannot get meme %d. Reason %s", smeme.MemeId, err)
		}

		res = append(res, MemeStat{
			MemeId:     meme.MemeId,
			Public:     meme.Public,
			Platform:   meme.Platform,
			Pictures:   fmt.Sprintf("%v", meme.Pictures),
			Likes:      likes,
			Dislikes:   dislikes,
			KekIndex:   meme.calculateKekIndex(),
			TimeCoeff:  meme.calculateTimeCoeff(),
			GroupCoeff: meme.calculateGroupRating(),
		})
	}

	return res, nil
}
