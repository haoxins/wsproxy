package wsproxy

import . "github.com/pkg4go/assert"
import "golang.org/x/net/websocket"
import "net/http/httptest"
import "io/ioutil"
import "net/http"
import "net/url"
import "testing"
import "time"
import "fmt"
import "log"
import "io"

var devnull = log.New(ioutil.Discard, "", 0)

func echoHandler(ws *websocket.Conn) {
	io.Copy(ws, ws)
}

func TestWSProxy(t *testing.T) {
	a := A{t}

	go func() {
		time.Sleep(3 * time.Second)
		panic("hi")
	}()
	echoServer := http.NewServeMux()
	echoServer.Handle("/dst", websocket.Handler(echoHandler))

	queryAssert := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("foo") != "bar" {
			t.Errorf("missing url query")
		}
		echoServer.ServeHTTP(w, r)
	})
	backend := httptest.NewServer(queryAssert)
	defer backend.Close()
	backendURL, err := url.Parse(backend.URL)
	a.Nil(err)

	backendURL.Path = "/dst"
	proxy := httptest.NewServer(NewProxy(backendURL))
	defer proxy.Close()

	for _, data := range []string{"hello,", "world", "!"} {
		res, err := sendWSRequest(proxy.URL+"/ws?foo=bar", data, t)
		a.Nil(err)
		a.Equal(res, data)
	}
}

func TestProxy(t *testing.T) {
	a := A{t}

	h := http.NewServeMux()
	h.Handle("/ws", websocket.Handler(func(ws *websocket.Conn) {
		ws.Write([]byte("ws success"))
		ws.Close()
	}))
	h.HandleFunc("/http", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("http success"))
	})
	isWSHandler := func(w http.ResponseWriter, r *http.Request) {
		isWS := IsWebSocketRequest(r)
		if isWS && (r.URL.Path != "/ws") {
			t.Errorf("ws path %s", r.URL.Path)
		} else if !isWS && (r.URL.Path != "/http") {
			t.Errorf("http path %s", r.URL.Path)
		}
		h.ServeHTTP(w, r)
	}
	n := httptest.NewServer(http.HandlerFunc(isWSHandler))
	defer n.Close()
	errc := make(chan error)
	go func() {
		res, err := http.Get(n.URL + "/http")
		if err != nil {
			errc <- fmt.Errorf("could't GET url: %v", err)
			return
		}
		defer res.Body.Close()
		data, err := ioutil.ReadAll(res.Body)
		if err != nil {
			errc <- fmt.Errorf("could't read body")
			return
		}
		t.Logf("http response: %s", data)
		if string(data) != "http success" {
			errc <- fmt.Errorf("expected 'http success', got '%s'", string(data))
			return
		}
		errc <- nil
	}()
	select {
	case err := <-errc:
		a.Nil(err)
	case <-time.After(4 * time.Second):
		t.Error("http request timedout")
	}
	go func() {
		t.Logf("making request to server")
		wsRes, err := sendWSRequest(n.URL+"/ws", "hello", t)
		if err != nil {
			errc <- fmt.Errorf("could't connect to ws server: %v", err)
			return
		}
		t.Logf("server response: %s", wsRes)
		if wsRes != "ws success" {
			errc <- fmt.Errorf("expected 'ws success' got '%s'", wsRes)
			return
		}
		errc <- nil
	}()
	t.Logf("waiting for websocket response")
	select {
	case err := <-errc:
		a.Nil(err)
		return
	case <-time.After(4 * time.Second):
		t.Error("websocket request timedout")
		return
	}
}

func TestHttpReq(t *testing.T) {
	a := A{t}

	handler := func(w http.ResponseWriter, r *http.Request) {
		t.Error("non-websocket request was made through proxy")
	}
	backend := httptest.NewServer(http.HandlerFunc(handler))
	defer backend.Close()

	u, e := url.Parse(backend.URL + "/")
	a.Nil(e)

	u.Scheme = "ws"

	proxy := NewProxy(u)
	proxy.ErrorLog = devnull
	proxyServer := httptest.NewServer(proxy)
	defer proxyServer.Close()

	_, e = http.Get(proxyServer.URL + "/")
	a.Nil(e)
}

func sendWSRequest(urlstr, data string, t *testing.T) (string, error) {
	a := A{t}

	a.NotEqual(data, "", "can't send empty data")

	u, e := url.Parse(urlstr)
	a.Nil(e)

	u.Scheme = "ws"
	origin := "http://localhost/"
	errc := make(chan error)
	wsc := make(chan *websocket.Conn)
	go func() {
		ws, e := websocket.Dial(u.String(), "", origin)
		if e != nil {
			errc <- e
			return
		}
		wsc <- ws
	}()
	var ws *websocket.Conn
	select {
	case e := <-errc:
		return "", e
	case ws = <-wsc:
	case <-time.After(time.Second * 2):
		return "", fmt.Errorf("websocket dial timedout")
	}
	defer ws.Close()
	msgc := make(chan string)
	go func() {
		if _, e := ws.Write([]byte(data)); e != nil {
			errc <- e
			return
		}
		var msg = make([]byte, 512)
		var n int
		if n, e = ws.Read(msg); e != nil {
			errc <- e
			return
		}
		msgc <- string(msg[:n])
	}()
	select {
	case e := <-errc:
		return "", e
	case msg := <-msgc:
		t.Logf("ws response: '%s'", msg)
		return msg, nil
	case <-time.After(time.Second * 2):
		return "", fmt.Errorf("websocket request timedout")
	}
}
