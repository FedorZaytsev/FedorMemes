package main

import (
  "net/http"
  "time"
  "encoding/json"
  //"strings"
  "fmt"

  pb "github.com/FedorZaytsev/FedorMemes"
)

type Server struct {
  apis []*APIConnect
  storage pb.StorageClient
}

func (s *Server) SendTop(wr http.ResponseWriter, req *http.Request) {
  memes, err := storage.GetUnshownMemes(Config.TelegramBot.ChatId, time.Now().Add(-time.Duration(24)*time.Hour))
	if err != nil {
		Log.Errorf("Cannot get memes. Reason %s", err)
		wr.WriteHeader(http.StatusInternalServerError)
		return
	}

	if len(memes) == 0 {
		Log.Errorf("No memes available")
		wr.WriteHeader(http.StatusInternalServerError)
		return
	}

	topMem := Meme{
    Meme: memes[0],
  }
	for _, memI := range memes {
    mem := Meme{
      Meme: memI,
    }
		if mem.СalculateKekScore() > topMem.СalculateKekScore() {
			topMem = mem
		}
	}

	public := topMem.Public
	/*switch strings.ToLower(topMem.Platform) {
	case "vk":
		public = Config.VK.Publics[topMem.Public].Name
	case "reddit":
		public = fmt.Sprintf("/r/%s", topMem.Public)
	}*/

	Log.Infof("Top mem: %v", topMem)

	msgid, err := Config.TelegramBot.SendPhoto(topMem.Pictures, topMem.Description,
		fmt.Sprintf("Новый мем от %s с индексом кекабельности %.2f",
			public,
			topMem.СalculateKekScore(),
		))
	if err != nil {
		Log.Errorf("Cannot send photo to telegram. Reason %s", err)
		wr.WriteHeader(http.StatusInternalServerError)
		return
	}

	err = storage.MarkMemeShown(Config.TelegramBot.ChatId, int32(msgid), topMem.Id)
	if err != nil {
		Log.Errorf("Cannot mark meme shown. Reason %s", err)
		wr.WriteHeader(http.StatusInternalServerError)
		return
	}

	memeStr, _ := json.MarshalIndent(MemeDebug{
		Meme:          topMem.Meme,
		KekIndex:      topMem.calculateKekIndex(),
		TimePassed:    time.Now().Sub(time.Unix(topMem.Time, 0)).String(),
		TimeCoeff:     topMem.calculateTimeCoeff(),
		GroupCoeff:    topMem.calculateGroupRating(),
		GroupActivity: topMem.calculateGroupActivity(),
		KekScore:      topMem.СalculateKekScore(),
	}, "", "  ")

	err = Config.TelegramBot.SendDebugText(fmt.Sprintf("Мем:\n%s", string(memeStr)))
	if err != nil {
		Log.Errorf("Cannot send debug info. Reason %s", err)
	}
}
