package main

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/cjongseok/mtproto"
	"github.com/go-chi/chi"

	pb "github.com/FedorZaytsev/FedorMemes"
)

type Telegram struct {
	AppID           int32
	AppHash         string
	PhoneNumber     string
	IP              string
	Port            int
	LookingDuration int
	LoadStep        int32
	caller          mtproto.RPCaller
}

type TelegramChannel struct {
	ChanName   string
	ChanId     int32
	AccessHash int64
}

func (t *Telegram) PrintDialogs() error {
	emptyPeer := &mtproto.TypeInputPeer{Value: &mtproto.TypeInputPeer_InputPeerEmpty{&mtproto.PredInputPeerEmpty{}}}
	resp, err := t.caller.MessagesGetDialogs(context.Background(), &mtproto.ReqMessagesGetDialogs{
		OffsetDate: 0, OffsetId: 0, OffsetPeer: emptyPeer, Limit: 20,
	})
	if err != nil {
		return fmt.Errorf("Cannot print dialogs. Reason %s", err)
	}
	switch dialogs := resp.GetValue().(type) {
	case *mtproto.TypeMessagesDialogs_MessagesDialogs:
		Log.Infof("Print dialogs: %v", dialogs.MessagesDialogs.String())
	case *mtproto.TypeMessagesDialogs_MessagesDialogsSlice:
		Log.Infof("Print dialogs: %v", dialogs.MessagesDialogsSlice.String())
	}

	return nil
}

func (t *Telegram) PrintMessages(chanId int32, accessHash int64) error {
	Log.Infof("MessagesGetHistory calling...")
	resp, err := t.caller.MessagesGetHistory(context.Background(), &mtproto.ReqMessagesGetHistory{
		Peer: &mtproto.TypeInputPeer{Value: &mtproto.TypeInputPeer_InputPeerChannel{
			&mtproto.PredInputPeerChannel{
				ChannelId:  chanId,
				AccessHash: accessHash,
			}},
		},
		Limit: 10,
	})
	Log.Infof("MessagesGetHistory called")
	if err != nil {
		return fmt.Errorf("Cannot print messages. Reason %s", err)
	}

	switch messages := resp.GetValue().(type) {
	case *mtproto.TypeMessagesMessages_MessagesMessages:
		Log.Infof("Print messages: %v", messages)
	case *mtproto.TypeMessagesMessages_MessagesMessagesSlice:
		Log.Infof("Print messages: %v", messages)
	case *mtproto.TypeMessagesMessages_MessagesChannelMessages:
		Log.Infof("Print messages: %v", messages)
	default:
		Log.Infof("Unknown type for messages. %v", messages)
	}

	return nil
}

func (t *Telegram) getMessagesFromChannel(addOffset int32, limit int32, channel TelegramChannel) ([]*mtproto.TypeMessage, error) {
	resp, err := t.caller.MessagesGetHistory(context.Background(), &mtproto.ReqMessagesGetHistory{
		Peer: &mtproto.TypeInputPeer{Value: &mtproto.TypeInputPeer_InputPeerChannel{
			&mtproto.PredInputPeerChannel{
				ChannelId:  channel.ChanId,
				AccessHash: channel.AccessHash,
			}},
		},
		Limit:     limit,
		AddOffset: addOffset,
	})
	if err != nil {
		return nil, fmt.Errorf("Cannot print messages. Reason %s", err)
	}

	switch messages := resp.GetValue().(type) {
	case *mtproto.TypeMessagesMessages_MessagesChannelMessages:
		return messages.MessagesChannelMessages.GetMessages(), nil
	default:
		return nil, fmt.Errorf("Unknown messages type")
	}
}

func (t *Telegram) getChannels() ([]TelegramChannel, error) {
	result := []TelegramChannel{}

	emptyPeer := &mtproto.TypeInputPeer{Value: &mtproto.TypeInputPeer_InputPeerEmpty{&mtproto.PredInputPeerEmpty{}}}
	resp, err := t.caller.MessagesGetDialogs(context.Background(), &mtproto.ReqMessagesGetDialogs{
		OffsetDate: 0, OffsetId: 0, OffsetPeer: emptyPeer, Limit: 100,
	})
	if err != nil {
		return result, fmt.Errorf("Cannot print dialogs. Reason %s", err)
	}

	switch dialogs := resp.GetValue().(type) {
	case *mtproto.TypeMessagesDialogs_MessagesDialogs:
		for _, chat := range dialogs.MessagesDialogs.GetChats() {
			channel := chat.GetChannel()
			if channel == nil {
				continue
			}
			result = append(result, TelegramChannel{
				ChanId:     channel.GetId(),
				ChanName:   channel.GetTitle(),
				AccessHash: channel.GetAccessHash(),
			})
		}
	case *mtproto.TypeMessagesDialogs_MessagesDialogsSlice:
		for _, chat := range dialogs.MessagesDialogsSlice.GetChats() {
			channel := chat.GetChannel()
			if channel == nil {
				continue
			}
			result = append(result, TelegramChannel{
				ChanId:     channel.GetId(),
				ChanName:   channel.GetTitle(),
				AccessHash: channel.GetAccessHash(),
			})
		}
	}

	return result, nil
}

func (t *Telegram) updateMemes(from time.Time) ([]*pb.Meme, error) {
	result := []*pb.Meme{}
	channels, err := t.getChannels()
	if err != nil {
		return result, fmt.Errorf("Cannot get channels. Reason %s", err)
	}
	for _, ch := range channels {
		memes, err := t.updateMemesFromChannel(from, ch)
		if err != nil {
			return result, fmt.Errorf("Cannot get memes from channel. Reason %s", err)
		}
		for _, meme := range memes {
			result = append(result, meme)
		}
	}
	return result, nil
}

func (t *Telegram) updateMemesFromChannel(from time.Time, channel TelegramChannel) ([]*pb.Meme, error) {
	memes := []*pb.Meme{}

	skipCount := int32(0)
	for {
		msgs, err := t.getMessagesFromChannel(skipCount, t.LoadStep, channel)
		if err != nil {
			return memes, fmt.Errorf("Cannot get messages from channel %d. Reason %s", channel, err)
		}
		skipCount += t.LoadStep

		for _, msgI := range msgs {
			//if message is not a message from a channel then skip it
			msg := msgI.GetMessage()
			if msg == nil {
				continue
			}
			photoMsg := msg.GetMedia().GetMessageMediaPhoto()
			if photoMsg.GetPhoto() == nil {
				continue
			}
			if photoMsg.GetPhoto().GetPhoto() == nil {
				continue
			}

			date := time.Unix(int64(msg.GetDate()), 0)
			if date.Before(from) {
				return memes, nil
			}

			memes = append(memes, &pb.Meme{
				MemeId:      fmt.Sprintf("%d", msg.GetId()),
				Public:      channel.ChanName,
				Platform:    "telegram",
				Description: msg.GetMessage(),
				Likes:       0,
				Reposts:     0,
				Views:       int32(msg.GetViews()),
				Comments:    0,
				Time:        int64(msg.GetDate()),
				Pictures:    []string{fmt.Sprintf("%d", photoMsg.GetPhoto().GetPhoto().GetId())},
			})
		}
	}

	return memes, nil
}

func (t *Telegram) requestAuth(manager *mtproto.Manager) (*mtproto.Conn, error) {
	Log.Infof("Trying to auth...")
	conn, sentCode, err := manager.NewAuthentication(t.PhoneNumber, t.AppID, t.AppHash, t.IP, t.Port)
	if err != nil {
		return nil, fmt.Errorf("Cannot create new authentication. Reason %s", err)
	}

	//Setup small http server to recieve code
	tgauthcode := make(chan string, 1)
	server := &http.Server{Addr: Config.HttpAddress}
	router := chi.NewRouter()
	router.Get("/telegram/authcode/{code}", func(w http.ResponseWriter, req *http.Request) {
		code := chi.URLParam(req, "code")
		Log.Info("Sending code")
		tgauthcode <- code
		Log.Infof("Shutting down server")
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		server.Shutdown(ctx)
	})
	server.Handler = router

	err = server.ListenAndServe()
	if err != nil && err != http.ErrServerClosed {
		return nil, fmt.Errorf("Cannot create server for receiving code from tg. Reason %s", err)
	}

	// sign-in with the code from the user input
	smsCode := <-tgauthcode
	Log.Info("Got code")
	_, err = conn.SignIn(t.PhoneNumber, smsCode, sentCode.GetValue().PhoneCodeHash)
	if err != nil {
		return nil, fmt.Errorf("Cannot sign into telegram. Reason %s", err)
	}
	return conn, nil
}

func (t *Telegram) Init() error {
	//slog.DisableLogging()
	mtconfig, err := mtproto.NewConfiguration("1.0", "fedormemes", "1.0", "ru", 0, 0, "./credentials.mtproto")
	if err != nil {
		return fmt.Errorf("Cannot configure mtproto. Reason %s", err)
	}

	manager, err := mtproto.NewManager(mtconfig)
	if err != nil {
		return fmt.Errorf("Cannot create new mtproto manager. Reason %s", err)
	}

	var conn *mtproto.Conn
	conn, err = manager.LoadAuthentication()
	if err != nil {
		var err2 error
		conn, err2 = t.requestAuth(manager)
		if err2 != nil {
			return fmt.Errorf("Cannot auth. Error while loading auth: %s. Error while requesting for auth: %s", err, err2)
		}
	}
	t.caller = mtproto.RPCaller{conn}

	return nil
}
