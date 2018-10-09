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

func (s *Storage) CalculateCounter(platform string, chatId int64, messageId int) (map[int32]int32, error) {
	btnCounters := make(map[int32]int32)
	rows, err := s.DB.Query("SELECT btn_id, count(*) FROM chat_metadata WHERE msg_id = ? and chat_id = ? GROUP BY btn_id", messageId, chatId)
	if err != nil {
		return btnCounters, fmt.Errorf("Cannot get counter for message %d. Reason %s", messageId, err)
	}
	defer rows.Close()
	for rows.Next() {
		var btnId int32
		var count int32
		err = rows.Scan(&btnId, &count)
		if err != nil {
			return btnCounters, fmt.Errorf("Cannot scan from row. Reason %s", err)
		}
		btnCounters[btnId] = count
	}

	return btnCounters, nil
}
