package main

import (
	"context"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	pb "github.com/FedorZaytsev/FedorMemes"
	"github.com/grpc-ecosystem/grpc-gateway/runtime"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
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

	storage, err = NewStorage(Config.DB.Name)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func main() {

	listener, err := net.Listen("tcp", Config.GrpcAddress)
	if err != nil {
		Log.Errorf("Cannot listen to port %s. Reason %s", Config.GrpcAddress, err)
		os.Exit(1)
	}
	grpcserv := grpc.NewServer()
	srv := Server{}

	pb.RegisterStorageServer(grpcserv, srv)
	reflection.Register(grpcserv)

	go func() {
		if err := grpcserv.Serve(listener); err != nil {
			Log.Errorf("Cannot serve. Reason %s", err)
			os.Exit(1)
		}
	}()

	go func() {
		mux := runtime.NewServeMux()
		err = pb.RegisterConsumerAPIHandlerFromEndpoint(context.Background(), mux,
			Config.GrpcAddress, []grpc.DialOption{grpc.WithInsecure()})
		if err != nil {
			Log.Error("Cannot register api hander. Reason %s", err)
			os.Exit(1)
		}
		http.ListenAndServe(Config.HttpAddress, mux)
	}()

	sgnl := make(chan os.Signal)
	signal.Notify(sgnl,
		syscall.SIGHUP,
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGQUIT)
	_ = <-sgnl
}
