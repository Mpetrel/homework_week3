package main

import (
	"context"
	"fmt"
	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func newAppServer() *http.Server {
	mux := http.DefaultServeMux
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprintln(w, "Hello world!")
	})
	return &http.Server{
		Addr: ":8080",
		Handler: mux,
	}
}

func newDebugServer() *http.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprintln(w, "Debug Server!")
	})
	return &http.Server{
		Addr: ":8081",
		Handler: mux,
	}
}

func main() {
	group, ctx := errgroup.WithContext(context.Background())

	serverClose := make(chan bool)
	// 创建两个server
	appServer := newAppServer()
	debugServer := newDebugServer()

	// 启动http服务
	group.Go(func() error {
		return appServer.ListenAndServe()
	})
	group.Go(func() error {
		return debugServer.ListenAndServe()
	})

	// 监听group完成事件，关闭服务
	group.Go(func() error {
		select {
		case <- ctx.Done():
			log.Printf("errgroup finished, close server..")
		case <- serverClose:
			log.Printf("receive custom close signal, close server...")
		}
		timeoutCtx, cancel := context.WithTimeout(context.Background(), 5 * time.Second)
		defer cancel()
		err := appServer.Shutdown(timeoutCtx)
		errDebug := debugServer.Shutdown(timeoutCtx)
		if err != nil {
			return err
		}
		return errDebug
	})

	// 监听系统事件
	group.Go(func() error {
		c := make(chan os.Signal)
		signal.Notify(c, os.Interrupt, os.Kill, syscall.SIGTERM, syscall.SIGINT)
		select {
		case <- ctx.Done():
			return ctx.Err()
		case sig := <- c:
			return errors.Errorf("receive system signal: %v", sig)
		}
	})

	// 模拟10秒后主动关闭服务
	time.AfterFunc(10 * time.Second, func() {
		log.Printf("close server as scheduled")
		serverClose <- true
	})

	if err := group.Wait(); err != nil {
		log.Fatal(err)
	}
}