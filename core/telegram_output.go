package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"gitlab.com/toby3d/telegram"
	"strconv"
	"strings"

	"time"
)

const MEDIA_CAPTION_SIZE = 200

type TelegramBot struct {
	Token       string
	bot         *telegram.Bot
	ChatId      int64
	ChatIdDebug int64
	updateId    int
	ch          chan telegram.Update
}

type InlineButtonData struct {
	Text    string `json:"text"`
	Counter int32    `json:"count"`
	IsMain  bool   `json:"is_main"`
}

func (b *InlineButtonData) Pack() []byte {
	return []byte(fmt.Sprintf("%s;%d", b.Text, b.Counter))
}

func (b *InlineButtonData) Unpack(data string) error {
	d := strings.Split(data, ";")
	if len(d) != 2 {
		return fmt.Errorf("Wrong data size. data == %s, size == %d", data, len(d))
	}
	b.Text = d[0]
	counter, err := strconv.Atoi(d[1])
	if err != nil {
		return fmt.Errorf("Cannot convert counter to int. Counter %s. Reason %s", d[1], err)
	}
	b.Counter = int32(counter)

	return nil
}

func HumanTime(t time.Time) string {
	timeNow := time.Now()
	if timeNow.Sub(t) < (time.Duration(5) * time.Minute) {
		return "только что"
	}
	if timeNow.Sub(t) < (time.Duration(7) * time.Hour) {
		hours := timeNow.Sub(t) / time.Hour
		if hours == 0 {
			return "меньше часа назад"
		} else if hours == 1 {
			return "час назад"
		} else if hours >= 2 && hours <= 4 {
			return fmt.Sprintf("%d часа назад", hours)
		} else {
			return fmt.Sprintf("%d часов назад", hours)
		}
	}
	if t.Day() == timeNow.Day() {
		hours := t.Hour()
		if hours == 0 {
			return "сегодня в полночь"
		} else if hours == 1 {
			return "в час ночи"
		} else if hours >= 2 && hours <= 4 {
			return fmt.Sprintf("в %d часа ночи", hours)
		} else if hours == 5 {
			return fmt.Sprintf("в %d часов ночи", hours)
		} else if hours > 5 && hours <= 10 {
			return fmt.Sprintf("в %d часов утра", hours)
		} else if hours > 10 && hours <= 12 {
			return fmt.Sprintf("в %d часов", hours)
		} else if hours > 12 && hours <= 20 {
			return fmt.Sprintf("в %d часов", hours-12)
		} else if hours > 20 && hours <= 23 {
			return fmt.Sprintf("в %d часов вечера", hours-12)
		} else {
			return fmt.Sprintf("в %d часов", hours)
		}
	}
	temp := t.AddDate(0, 0, 1)
	if timeNow.Year() == temp.Year() && timeNow.Month() == temp.Month() && timeNow.Day() == temp.Day() {
		hours := t.Hour()
		if hours == 0 {
			return "вчера в полночь"
		} else if hours == 1 {
			return "вчера в час ночи"
		} else if hours >= 2 && hours <= 4 {
			return fmt.Sprintf("вчера в %d часа ночи", hours)
		} else if hours == 5 {
			return fmt.Sprintf("вчера в %d часов ночи", hours)
		} else if hours > 5 && hours <= 10 {
			return fmt.Sprintf("вчера в %d часов утра", hours)
		} else if hours > 10 && hours <= 12 {
			return fmt.Sprintf("вчера в %d часов", hours)
		} else if hours > 12 && hours <= 20 {
			return fmt.Sprintf("вчера в %d часов", hours-12)
		} else if hours > 20 && hours <= 23 {
			return fmt.Sprintf("вчера в %d часов вечера", hours-12)
		} else {
			return fmt.Sprintf("вчера в %d часов", hours)
		}
	}
	return t.Format(time.RFC3339)
}

func packMetadata(data []InlineButtonData, mainIndex int) string {
	if len(data) == 0 {
		return ""
	}
	buf := bytes.NewBuffer([]byte{})
	buf.Write([]byte(strconv.Itoa(mainIndex)))
	buf.Write([]byte("|"))
	for _, btn := range data {
		buf.Write(btn.Pack())
		buf.Write([]byte("|"))
	}
	res := buf.String()
	return res[:len(res)-1]
}

func unpackMetadata(data string) ([]InlineButtonData, int, error) {
	var err error
	btns := []InlineButtonData{}
	mainIndex := 0
	d := strings.Split(data, "|")
	if len(d) < 2 {
		return btns, mainIndex, fmt.Errorf("Cannot unpack data. Reason: data is too small %d", len(d))
	}
	mainIndex, err = strconv.Atoi(d[0])
	if err != nil {
		return btns, mainIndex, fmt.Errorf("Cannot parse counter %s. Reason %s", d[0], err)
	}
	for i := 1; i < len(d); i++ {
		btn := InlineButtonData{}
		err = btn.Unpack(d[i])
		if err != nil {
			return btns, mainIndex, fmt.Errorf("Cannot unpack button. Reason %s", err)
		}
		btns = append(btns, btn)
	}
	return btns, mainIndex, nil
}

func NewInlineKeyboardCounter(data []InlineButtonData) *telegram.InlineKeyboardMarkup {
	keyboardRow := []telegram.InlineKeyboardButton{}
	for i, entry := range data {
		serialized := packMetadata(data, i)
		keyboardRow = append(keyboardRow, telegram.NewInlineKeyboardButton(fmt.Sprintf("%s %d", entry.Text, entry.Counter), string(serialized)))
	}

	return telegram.NewInlineKeyboardMarkup(keyboardRow)
}

func (b *TelegramBot) Connect() error {
	var err error
	b.bot, err = telegram.New(b.Token)
	if err != nil {
		return fmt.Errorf("Cannot connect to tg. Reason %s", err)
	}

	go b.EventHandler()

	return b.Init()
}

func (b *TelegramBot) EventHandler() {
	for update := range b.ch {
		if update.CallbackQuery != nil {
			Log.Infof("CallbackQuery %v", update.CallbackQuery)
			Log.Infof("CallbackQuery.MSG %v", update.CallbackQuery.Message)

			metadata, mainIndex, err := unpackMetadata(update.CallbackQuery.Data)
			if err != nil {
				Log.Errorf("Cannot process update. Reason %s", err)
				continue
			}

			err = storage.MakeAction("telegram",
				update.CallbackQuery.Message.Chat.ID,
				update.CallbackQuery.Message.ID,
				update.CallbackQuery.From.ID,
				mainIndex)
			if err != nil {
				Log.Errorf("Cannot make action. Reason %s", err)
				continue
			}

			counters, err := storage.CalculateCounter("telegram", update.CallbackQuery.Message.Chat.ID, update.CallbackQuery.Message.ID)
			if err != nil {
				Log.Errorf("Cannot calculate counters for messages. Reason %s", err)
			}

			for i := 0; i < len(metadata); i++ {
				metadata[i].Counter = counters[int32(i)]
			}

			keyboard := telegram.EditMessageReplyMarkupParameters{
				ChatID:      update.CallbackQuery.Message.Chat.ID,
				MessageID:   update.CallbackQuery.Message.ID,
				ReplyMarkup: NewInlineKeyboardCounter(metadata),
			}
			_, err = b.bot.EditMessageReplyMarkup(&keyboard)
			if err != nil {
				Log.Errorf("Cannot edit message for updating keyboard message. Reason %s", err)
			}

			b.bot.AnswerCallbackQuery(&telegram.AnswerCallbackQueryParameters{
				CallbackQueryID: update.CallbackQuery.ID,
			})
		}
		if update.Message != nil {
			Log.Infof("Got new message in chat: %v", update.Message)
		}
	}
}

func (b *TelegramBot) SendTextMessage(text string) error {
	msg := telegram.NewMessage(b.ChatId, text)

	_, err := b.bot.SendMessage(msg)

	return err
}

func (b *TelegramBot) SendDebugText(text string) error {
	msg := telegram.NewMessage(b.ChatIdDebug, text)
	msg.DisableWebPagePreview = true

	_, err := b.bot.SendMessage(msg)

	return err
}

func (b *TelegramBot) SendPhoto(paths []string, text, description string) (int, error) {
	btns := []InlineButtonData{
		InlineButtonData{
			Text:    "👍",
			Counter: 0,
		},
		InlineButtonData{
			Text:    "👎",
			Counter: 0,
		},
	}

	if len(paths) == 0 {
		return 0, fmt.Errorf("Cannot send photo. Reason: no photo")
	} else if len(paths) == 1 {
		caption := fmt.Sprintf("%s\n\n%s", text, description)
		if len(caption) >= MEDIA_CAPTION_SIZE {
			msg := telegram.NewPhoto(b.ChatId, paths[0])
			msg.DisableWebPagePreview = true
			res, err := b.bot.SendPhoto(msg)
			if err != nil {
				return 0, fmt.Errorf("Cannot send meme. Reason %s", err)
			}
			msgKeyboard := telegram.NewMessage(b.ChatId, caption)
			msgKeyboard.DisableWebPagePreview = true
			msgKeyboard.ReplyMarkup = NewInlineKeyboardCounter(btns)
			res, err = b.bot.SendMessage(msgKeyboard)
			if err != nil {
				return 0, fmt.Errorf("Cannot send message with keyboard. Reason %s", err)
			}
			return res.ID, nil
		} else {
			msg := telegram.NewPhoto(b.ChatId, paths[0])
			msg.Caption = fmt.Sprintf("%s\n\n%s", text, description)
			msg.DisableWebPagePreview = true
			msg.ReplyMarkup = NewInlineKeyboardCounter(btns)
			res, err := b.bot.SendPhoto(msg)
			if err != nil {
				return 0, fmt.Errorf("Cannot send meme. Reason %s", err)
			}

			return res.ID, nil
		}

	} else {
		media := []interface{}{}
		for i, path := range paths {
			ph := telegram.NewInputMediaPhoto(path)
			if i == 0 {
				ph.Caption = text
			}
			media = append(media, interface{}(ph))
		}
		Log.Infof("Sending %d photos", len(media))

		_, err := b.bot.SendMediaGroup(&telegram.SendMediaGroupParameters{
			ChatID: b.ChatId,
			Media:  media,
		})
		if err != nil {
			return 0, fmt.Errorf("Cannot send media group. Reason %s", err)
		}

		msgKeyboard := telegram.NewMessage(b.ChatId, description)
		msgKeyboard.DisableWebPagePreview = true
		msgKeyboard.ReplyMarkup = NewInlineKeyboardCounter(btns)
		res, err := b.bot.SendMessage(msgKeyboard)
		if err != nil {
			return 0, fmt.Errorf("Cannot send message with keyboard. Reason %s", err)
		}
		return res.ID, nil
	}
}

func (b *TelegramBot) SendPhotoViaURL(address string) error {
	return b.SendTextMessage(address)
}

func (b *TelegramBot) Init() error {
	b.ch = make(chan telegram.Update)
	go func() {
		for {
			updates, err := b.bot.GetUpdates(&telegram.GetUpdatesParameters{
				Offset:  b.updateId,
				Timeout: 60,
			})
			Log.Infof("updates %v", updates)
			updatesStr, _ := json.Marshal(updates)
			if err != nil {
				Log.Errorf("Cannot recieve update from telegram. Reason %s", err)
				time.Sleep(time.Duration(3) * time.Second)
				continue
			}
			if len(updates) > 0 {
				Log.Infof("updates %s %s", updatesStr, err)
			}
			for _, update := range updates {
				if update.ID >= b.updateId {
					b.updateId = update.ID + 1
					b.ch <- update
				}
			}
		}
	}()

	return nil
}
