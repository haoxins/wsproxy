package wsproxy

import "crypto/tls"
import "net/http"
import "net/url"
import "strings"
import "log"
import "net"
import "io"

type Proxy struct {
	Director func(*http.Request)

	Dial func(network, addr string) (net.Conn, error)

	TLSClientConfig *tls.Config

	ErrorLog *log.Logger
}

func NewProxy(target *url.URL) *Proxy {
	targetQuery := target.RawQuery
	director := func(req *http.Request) {
		req.URL.Scheme = target.Scheme
		req.URL.Host = target.Host
		req.URL.Path = target.Path

		if targetQuery == "" || req.URL.RawQuery == "" {
			req.URL.RawQuery = targetQuery + req.URL.RawQuery
		} else {
			req.URL.RawQuery = targetQuery + "&" + req.URL.RawQuery
		}
	}

	return &Proxy{Director: director}
}

func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	logFunc := log.Printf
	if p.ErrorLog != nil {
		logFunc = p.ErrorLog.Printf
	}

	if !IsWebSocketRequest(r) {
		http.Error(w, "Can't handle non-WebSocket requests", 406)
		logFunc("Received non-WebSocket request")
		return
	}

	outreq := new(http.Request)
	// shallow copying
	*outreq = *r
	p.Director(outreq)
	host := outreq.URL.Host

	dial := p.Dial
	if dial == nil {
		dial = net.Dial
	}

	if !strings.Contains(host, ":") {
		if outreq.URL.Scheme == "wss" {
			host = host + ":443"
		} else {
			host = host + ":80"
		}
	}

	if outreq.URL.Scheme == "wss" {
		var tlsConfig *tls.Config
		if p.TLSClientConfig == nil {
			tlsConfig = &tls.Config{}
		} else {
			tlsConfig = p.TLSClientConfig
		}
		dial = func(network, address string) (net.Conn, error) {
			return tls.Dial("tcp", host, tlsConfig)
		}
	}

	d, err := dial("tcp", host)
	if err != nil {
		http.Error(w, "Error forwarding request.", 500)
		logFunc("Error dialing websocket backend %s: %v", outreq.URL, err)
		return
	}

	hj, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "Not a hijacker", 500)
		return
	}

	nc, _, err := hj.Hijack()
	if err != nil {
		logFunc("Hijack error: %v", err)
		return
	}
	defer nc.Close()
	defer d.Close()

	err = outreq.Write(d)
	if err != nil {
		logFunc("Error copying request to target: %v", err)
		return
	}
	errc := make(chan error, 2)
	cp := func(dst io.Writer, src io.Reader) {
		_, err := io.Copy(dst, src)
		errc <- err
	}
	go cp(d, nc)
	go cp(nc, d)
	<-errc
}
