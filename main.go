package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	_ "net/http/pprof"
	"os"
	"strings"
	"time"

	"github.com/go-chi/chi"
	"github.com/gocarina/gocsv"
	"github.com/ogier/pflag"
)

var (
	Version, BuildTime string
)

func init() {
	var versReq bool
	var configPath string
	pflag.StringVarP(&configPath, "config", "c", "config.toml", "Used for set path to config file.")
	pflag.BoolVarP(&versReq, "version", "v", false, "Use for build time and version print")
	var err error
	pflag.Parse()
	if versReq {
		fmt.Println("Version: ", Version)
		fmt.Println("Build time:", BuildTime)
		os.Exit(0)
	}
	Config, err = getConfig(configPath)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	Log, err = initLogger()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	err = Config.Reddit.Init()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	err = Config.TelegramBot.Connect()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	storage, err = NewStorage(Config.DB.Name)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	err = storage.Init()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	err = Config.VK.Init()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func updateMemes(wr http.ResponseWriter, req *http.Request) {
	Config.VK.update()
	Config.Reddit.update()
	err := storage.Dump()
	if err != nil {
		Log.Errorf("Cannot dump memes. Reason %s", err)
	}
}

func downloadFile(wr http.ResponseWriter, req *http.Request, f *os.File) {
	stat, err := f.Stat()
	if err != nil {
		Log.Errorf("Cannot get stat for dump.csv. Reason %s", err)
		wr.WriteHeader(http.StatusInternalServerError)
		return
	}

	wr.Header().Set("Content-Disposition", "attachment; filename=dump.csv")
	wr.Header().Set("Content-Type", "text/csv")
	wr.Header().Set("Cache-Control", "no-cache")
	wr.Header().Set("Content-Length", fmt.Sprintf("%d", stat.Size()))

	_, err = io.Copy(wr, f)
	if err != nil {
		Log.Errorf("Cannot copy dump.csv. Reason %s", err)
		wr.WriteHeader(http.StatusInternalServerError)
		return
	}
}

func downloadDump(wr http.ResponseWriter, req *http.Request) {
	f, err := os.Open("./dump.csv")
	if err != nil {
		Log.Errorf("Cannot read memes dump. Reason %s", err)
		wr.WriteHeader(http.StatusInternalServerError)
		return
	}
	downloadFile(wr, req, f)
}

func downloadStats(wr http.ResponseWriter, req *http.Request) {
	stats, err := storage.getStatistics(Config.TelegramBot.ChatId)
	if err != nil {
		Log.Errorf("Cannot get statistic. Reason %s", err)
		wr.WriteHeader(http.StatusInternalServerError)
		return
	}

	csvContent, err := gocsv.MarshalString(&stats)
	if err != nil {
		Log.Errorf("Cannot marshal to csv. Reason %s", err)
		wr.WriteHeader(http.StatusInternalServerError)
		return
	}

	r := strings.NewReader(csvContent)

	wr.Header().Set("Content-Disposition", "attachment; filename=stats.csv")
	wr.Header().Set("Content-Type", "text/csv")
	wr.Header().Set("Cache-Control", "no-cache")
	wr.Header().Set("Content-Length", fmt.Sprintf("%d", r.Size()))

	_, err = io.Copy(wr, r)
	if err != nil {
		Log.Errorf("Cannot copy stats.csv. Reason %s", err)
		wr.WriteHeader(http.StatusInternalServerError)
		return
	}
}

func downloadRatings(wr http.ResponseWriter, req *http.Request) {

	stats, err := storage.CalculateGroupRating(Config.TelegramBot.ChatId)
	if err != nil {
		Log.Errorf("Cannot get groups ratings. Reason %s", err)
		wr.WriteHeader(http.StatusInternalServerError)
		return
	}

	type csvStats struct {
		Platform string
		Public   string
		Rating   float64
	}

	temp := []csvStats{}
	for platform, el := range stats {
		for public, rating := range el {
			temp = append(temp, csvStats{
				Public:   public,
				Platform: platform,
				Rating:   rating,
			})
		}
	}

	csvContent, err := gocsv.MarshalString(&temp)
	if err != nil {
		Log.Errorf("Cannot marshal to csv. Reason %s", err)
		wr.WriteHeader(http.StatusInternalServerError)
		return
	}

	r := strings.NewReader(csvContent)

	wr.Header().Set("Content-Disposition", "attachment; filename=ratings.csv")
	wr.Header().Set("Content-Type", "text/csv")
	wr.Header().Set("Cache-Control", "no-cache")
	wr.Header().Set("Content-Length", fmt.Sprintf("%d", r.Size()))

	_, err = io.Copy(wr, r)
	if err != nil {
		Log.Errorf("Cannot copy stats.csv. Reason %s", err)
		wr.WriteHeader(http.StatusInternalServerError)
		return
	}
}

func topDaylyMemHandler(wr http.ResponseWriter, req *http.Request) {
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
	Log.Infof("Available memes %v", memes)

	topMem := memes[0]
	for _, mem := range memes {
		if mem.Platform == "reddit" {
			Log.Infof("found meme from reddit with kek score %f. Top is %f", mem.calculateKekScore(), topMem.calculateKekScore())
		}
		if mem.calculateKekScore() > topMem.calculateKekScore() {
			topMem = mem
		}
	}

	public := ""
	switch strings.ToLower(topMem.Platform) {
	case "vk":
		public = Config.VK.Publics[topMem.Public].Name
	case "reddit":
		public = fmt.Sprintf("/r/%s", topMem.Public)
	}

	Log.Infof("Top mem: %v", topMem)

	msgid, err := Config.TelegramBot.SendPhoto(topMem.Pictures, topMem.Description,
		fmt.Sprintf("Новый мем от %s с индексом кекабельности %.2f",
			public,
			topMem.calculateKekScore(),
		))
	if err != nil {
		Log.Errorf("Cannot send photo to telegram. Reason %s", err)
		wr.WriteHeader(http.StatusInternalServerError)
		return
	}

	err = storage.MarkMemeShown(Config.TelegramBot.ChatId, msgid, topMem.Id)
	if err != nil {
		Log.Errorf("Cannot mark meme shown. Reason %s", err)
		wr.WriteHeader(http.StatusInternalServerError)
		return
	}

	memeStr, _ := json.MarshalIndent(MemeDebug{
		Meme:          topMem,
		KekIndex:      topMem.calculateKekIndex(),
		TimePassed:    time.Now().Sub(topMem.Time).String(),
		TimeCoeff:     topMem.calculateTimeCoeff(),
		GroupCoeff:    topMem.calculateGroupCoeff(),
		GroupActivity: topMem.calculateGroupActivity(),
		KekScore:      topMem.calculateKekScore(),
	}, "", "  ")

	err = Config.TelegramBot.SendDebugText(fmt.Sprintf("Мем:\n%s", string(memeStr)))
	if err != nil {
		Log.Errorf("Cannot send debug info. Reason %s", err)
	}

}

func main() {
	router := chi.NewRouter()
	router.Post("/post", topDaylyMemHandler)
	router.Post("/update/memes", updateMemes)
	router.Get("/download/dump", downloadDump)
	router.Get("/download/stats", downloadStats)
	router.Get("/download/ratings", downloadRatings)

	go func() {
		Log.Infof("pprof result %v", http.ListenAndServe(":6060", nil))
	}()

	err := http.ListenAndServe(Config.ServeAddress, router)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	err = storage.DB.Close()
	if err != nil {
		Log.Errorf("Cannot close db. Reason %s", err)
	}
}
