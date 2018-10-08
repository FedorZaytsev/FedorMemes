package main

import (
	"fmt"
	"reflect"

	"github.com/shelomentsevd/mtproto"
)

type Peer struct {
	UserID    int32
	ChatID    int32
	ChannelID int32
}

func NewPeer(peer mtproto.TL) (Peer, error) {
	switch pr := peer.(type) {
	case mtproto.TL_peerChat:
		return Peer{
			ChatID: pr.Chat_id,
		}, nil
	case mtproto.TL_peerUser:
		return Peer{
			UserID: pr.User_id,
		}, nil
	case mtproto.TL_peerChannel:
		return Peer{
			ChannelID: pr.Channel_id,
		}, nil
	default:
		return Peer{}, fmt.Errorf("Cannot parse peer field. Type %v", reflect.TypeOf(peer))
	}
}

type PeerNotifySettings struct {
	Flags        int32
	ShowPreviews bool
	Silent       bool
	MuteUntil    int32
	Sound        string
}

func NewPeerNotifySettings(pns mtproto.TL) (PeerNotifySettings, error) {
	switch notifications := pns.(type) {
	case mtproto.TL_peerNotifySettings:
		return PeerNotifySettings{
			Flags:        notifications.Flags,
			ShowPreviews: notifications.Show_previews,
			Silent:       notifications.Silent,
			MuteUntil:    notifications.Mute_until,
			Sound:        notifications.Sound,
		}, nil
	default:
		return PeerNotifySettings{}, fmt.Errorf("Cannot parse PeerNotifySettings field. Type %v", reflect.TypeOf(pns))
	}
}

type Dialog struct {
	Flags           int32
	Pinned          bool
	Peer            Peer
	TopMessage      int32
	ReadInboxMaxID  int32
	ReadOutboxMaxID int32
	UnreadCount     int32
	NotifySettings  PeerNotifySettings
	Pts             int32
	//Draft           DraftMessage
}

func NewDialog(dlgI mtproto.TL) (Dialog, error) {
	switch dlg := dlgI.(type) {
	case mtproto.TL_dialog:
		peer, err := NewPeer(dlg.Peer)
		if err != nil {
			return Dialog{}, fmt.Errorf("Cannot parse peer. Reason %s", err)
		}

		ns, err := NewPeerNotifySettings(dlg.Notify_settings)
		if err != nil {
			return Dialog{}, fmt.Errorf("Cannot parse notify setting. Reason %s", err)
		}

		return Dialog{
			Flags:           dlg.Flags,
			Pinned:          dlg.Pinned,
			Peer:            peer,
			TopMessage:      dlg.Top_message,
			ReadInboxMaxID:  dlg.Read_inbox_max_id,
			ReadOutboxMaxID: dlg.Read_outbox_max_id,
			UnreadCount:     dlg.Unread_count,
			NotifySettings:  ns,
			Pts:             dlg.Pts,
		}, nil
	default:
		return Dialog{}, fmt.Errorf("Cannot parse dialog field. Type %v", reflect.TypeOf(dlgI))
	}
}

type Dialogs struct {
	Count    int32
	Dialogs  []Dialog
	Messages []Message
	//Chats
	//Users
}

type FileLocation struct {
	DcID     int32
	VolumeID int64
	LocalID  int32
	Secret   int64
}

func NewFileLocation(fl mtproto.TL) (FileLocation, error) {
	switch loc := fl.(type) {
	case mtproto.TL_fileLocation:
		return FileLocation{
			DcID:     loc.Dc_id,
			VolumeID: loc.Volume_id,
			LocalID:  loc.Local_id,
			Secret:   loc.Secret,
		}, nil
	case mtproto.TL_fileLocationUnavailable:
		return FileLocation{
			VolumeID: loc.Volume_id,
			LocalID:  loc.Local_id,
			Secret:   loc.Secret,
		}, nil
	default:
		return FileLocation{}, fmt.Errorf("Cannot parse field file location. Type %v", reflect.TypeOf(fl))
	}
}

type PhotoSize struct {
	CodeType string
	Location FileLocation
	W        int32
	H        int32
	Size     int32
	Bytes    []byte
}

func NewPhotoSize(ps mtproto.TL) (PhotoSize, error) {
	switch size := ps.(type) {
	case mtproto.TL_photoSizeEmpty:
		return PhotoSize{
			CodeType: size.Code_type,
		}, nil
	case mtproto.TL_photoSize:
		loc, err := NewFileLocation(size.Location)
		if err != nil {
			return PhotoSize{}, fmt.Errorf("Cannot parse file location. Reason %s", err)
		}
		return PhotoSize{
			CodeType: size.Code_type,
			Location: loc,
			W:        size.W,
			H:        size.H,
			Size:     size.Size,
		}, nil
	case mtproto.TL_photoCachedSize:
		loc, err := NewFileLocation(size.Location)
		if err != nil {
			return PhotoSize{}, fmt.Errorf("Cannot parse file location. Reason %s", err)
		}
		return PhotoSize{
			CodeType: size.Code_type,
			Location: loc,
			W:        size.W,
			H:        size.H,
			Bytes:    size.Bytes,
		}, nil
	default:
		return PhotoSize{}, fmt.Errorf("Cannot parse field photo size. Type %v", reflect.TypeOf(size))
	}
}

type Photo struct {
	Flags       int32
	HasStickers bool
	ID          int64
	AccessHash  int64
	Date        int32
	Sizes       []PhotoSize
}

func NewPhoto(photoI mtproto.TL) (Photo, error) {
	switch photo := photoI.(type) {
	case mtproto.TL_photoEmpty:
		return Photo{
			ID: photo.Id,
		}, nil
	case mtproto.TL_photo:
		sizes := []PhotoSize{}
		for _, size := range photo.Sizes {
			s, err := NewPhotoSize(size)
			if err != nil {
				return Photo{}, fmt.Errorf("Cannot parse field size. Reason %s", err)
			}
			sizes = append(sizes, s)
		}
		return Photo{
			Flags:       photo.Flags,
			HasStickers: photo.Has_stickers,
			ID:          photo.Id,
			AccessHash:  photo.Access_hash,
			Date:        photo.Date,
			Sizes:       sizes,
		}, nil
	default:
		return Photo{}, fmt.Errorf("Cannot parse photo. Type %v", reflect.TypeOf(photo))
	}
}

const MessageMediaEmpty = 1
const MessageMediaPhoto = 2
const MessageMediaVideo = 3
const MessageMediaGeo = 4
const MessageMediaContact = 5
const MessageMediaDocument = 6
const MessageMediaAudio = 7

type MessageMedia struct {
	Type  int
	Photo Photo
}

func NewMessageMedia(mediaI mtproto.TL) (MessageMedia, error) {
	switch media := mediaI.(type) {
	case mtproto.TL_messageMediaEmpty:
		return MessageMedia{
			Type: MessageMediaEmpty,
		}, nil
	case mtproto.TL_messageMediaPhoto:
		photo, err := NewPhoto(media.Photo)
		if err != nil {
			return MessageMedia{}, fmt.Errorf("Cannot parse message photo media. Reason %s", err)
		}
		return MessageMedia{
			Type:  MessageMediaPhoto,
			Photo: photo,
		}, nil
	default:
		return MessageMedia{}, nil
	}
}

type MessageAction struct {
}

type Message struct {
	ID           int32
	FromID       int32
	ToID         Peer
	ViaBotId     int32
	ReplyToMsgID int32        // reply_to_msg_id:flags.3?int
	Date         int32        // date:int
	Message      string       // message:string
	Media        MessageMedia // media:flags.9?MessageMedia
	Views        int32        // views:flags.10?int
	EditDate     int32        // edit_date:flags.15?int
}

func NewMessage(msgI mtproto.TL) (Message, error) {
	switch msg := msgI.(type) {
	case mtproto.TL_messageEmpty:
		return Message{
			ID: msg.Id,
		}, nil
	case mtproto.TL_message:
		toID, err := NewPeer(msg.To_id)
		if err != nil {
			return Message{}, fmt.Errorf("Cannot parse To_id field. Reason %s", err)
		}
		media, err := NewMessageMedia(msg.Media)
		if err != nil {
			return Message{}, fmt.Errorf("Cannot parse Media field. Reason %s", err)
		}
		return Message{
			ID:           msg.Id,
			FromID:       msg.From_id,
			ToID:         toID,
			ViaBotId:     msg.Via_bot_id,
			ReplyToMsgID: msg.Reply_to_msg_id,
			Date:         msg.Date,
			Message:      msg.Message,
			Media:        media,
			Views:        msg.Views,
			EditDate:     msg.Edit_date,
		}, nil
	default:
		Log.Errorf("Cannot parse msg %v", msg)
		return Message{}, nil
	}
}
