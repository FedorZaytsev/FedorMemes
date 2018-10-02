package main

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/go-chi/chi"
	"github.com/gocarina/gocsv"
)

var (
	Version, BuildTime string
)

func init() {
	var versReq bool
	var configPath string
	flag.StringVar(&configPath, "c", "config.toml", "Used for set path to config file.")
	flag.BoolVar(&versReq, "v", false, "Use for build time and version print")
	var err error
	flag.Parse()
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

	err = Config.Telegram.Init()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func updateMemes(wr http.ResponseWriter, req *http.Request) {
	Config.VK.update()
	Config.Reddit.update()
	Config.Telegram.update()
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
	var err error
	id := chi.URLParam(req, "id")
	data := [][]string{}

	switch id {
	case "groupRatings":
		data = append(data, []string{"platform", "group", "rating"})
		for platform := range storage.GroupRatings {
			for group, rating := range storage.GroupRatings[platform] {
				data = append(data, []string{platform, group, fmt.Sprintf("%f", rating)})
			}
		}
	case "groupActivity":
		data = append(data, []string{"platform", "group", "activity"})
		for platform := range storage.GroupActivity {
			for group, activity := range storage.GroupActivity[platform] {
				data = append(data, []string{platform, group, fmt.Sprintf("%f", activity)})
			}
		}
	case "platformRatings":
		data = append(data, []string{"platform", "rating"})
		for platform, rating := range storage.PlatformRatings {
			data = append(data, []string{platform, fmt.Sprintf("%f", rating)})
		}
	case "platformActivity":
		data = append(data, []string{"platform", "rating"})
		for platform, activity := range storage.PlatformActivity {
			data = append(data, []string{platform, fmt.Sprintf("%f", activity)})
		}
	default:
		http.Error(wr, "Wrong id. Available groupRatings, groupActivity, platformRatings, platformActivity", http.StatusBadRequest)
		return
	}

	buf := bytes.NewBuffer([]byte{})

	err = csv.NewWriter(buf).WriteAll(data)
	if err != nil {
		Log.Errorf("Cannot marshal to csv. Reason %s", err)
		wr.WriteHeader(http.StatusInternalServerError)
		return
	}

	wr.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s.csv", id))
	wr.Header().Set("Content-Type", "text/csv")
	wr.Header().Set("Cache-Control", "no-cache")
	wr.Header().Set("Content-Description", "File Transfer")
	wr.Header().Set("Content-Length", fmt.Sprintf("%d", buf.Len()))

	_, err = io.Copy(wr, buf)
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
		if mem.СalculateKekScore() > topMem.СalculateKekScore() {
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
			topMem.СalculateKekScore(),
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
		GroupCoeff:    topMem.calculateGroupRating(),
		GroupActivity: topMem.calculateGroupActivity(),
		KekScore:      topMem.СalculateKekScore(),
	}, "", "  ")

	err = Config.TelegramBot.SendDebugText(fmt.Sprintf("Мем:\n%s", string(memeStr)))
	if err != nil {
		Log.Errorf("Cannot send debug info. Reason %s", err)
	}

}

func main() {
	var err error

	router := chi.NewRouter()
	router.Post("/post", topDaylyMemHandler)
	router.Post("/update/memes", updateMemes)
	router.Get("/download/dump", downloadDump)
	router.Get("/download/stats", downloadStats)
	router.Get("/download/ratings/{id}", downloadRatings)

	err = http.ListenAndServe(Config.ServeAddress, router)
	if err != nil {
		fmt.Println("ListenAndServe", err)
		os.Exit(1)
	}

	err = storage.DB.Close()
	if err != nil {
		Log.Errorf("Cannot close db. Reason %s", err)
	}
	sgnl := make(chan os.Signal)
	signal.Notify(sgnl,
		syscall.SIGHUP,
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGQUIT)
	_ = <-sgnl
}
