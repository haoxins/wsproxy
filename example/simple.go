package main

import "golang.org/x/net/websocket"
import "../../wsproxy"
import "net/http"
import "net/url"
import "time"
import "log"
import "io"

func main() {
	ori := ":3001"
	dst := ":3002"

	echoHandler := websocket.Handler(func(ws *websocket.Conn) {
		io.Copy(ws, ws)
	})

	u := &url.URL{Scheme: "ws://", Host: dst}
	p := wsproxy.NewProxy(u)

	go http.ListenAndServe(dst, echoHandler)
	go http.ListenAndServe(ori, p)

	time.Sleep(1 * time.Second)

	_, err := websocket.Dial("ws://"+ori, "", "http://localhost/")
	if err != nil {
		log.Fatalf("ws server error: %v", err)
	}

	quit := make(chan bool)
	if <-quit {
	}
}
