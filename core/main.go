package main

import (
	"flag"
	"fmt"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"syscall"

	"github.com/go-chi/chi"

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
}


func main() {
	var err error

	srv := Server{
		apis: []*APIConnect{},
	}

	for _, api := range Config.API {
		conn, err := NewApiConnect(api)
		if err != nil {
			Log.Errorf("Cannot init api. Reason %s", err)
			os.Exit(1)
		}
		srv.apis = append(srv.apis, conn)
	}

	storage, err = NewStorage(Config.Storage)
	if err != nil {
		Log.Errorf("Cannot create storage. Reason %s", err)
		os.Exit(1)
	}

	router := chi.NewRouter()
	router.Post("/post", srv.SendTop)
	//router.Post("/update/memes", updateMemes)
	//router.Get("/download/dump", downloadDump)
	//router.Get("/download/stats", downloadStats)
	//router.Get("/download/ratings/{id}", downloadRatings)

	err = http.ListenAndServe(Config.ServeAddress, router)
	if err != nil {
		fmt.Println("ListenAndServe", err)
		os.Exit(1)
	}

	sgnl := make(chan os.Signal)
	signal.Notify(sgnl,
		syscall.SIGHUP,
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGQUIT)
	_ = <-sgnl
}
