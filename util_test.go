package wsproxy

import . "github.com/pkg4go/assert"
import "net/http/httptest"
import "io/ioutil"
import "net/http"
import "testing"
import "strconv"
import "fmt"

func TestIsWebSocketRequest(t *testing.T) {
	a := A{t}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, strconv.FormatBool(IsWebSocketRequest(r)))
	}))
	defer ts.Close()

	// not ws
	res1, err := http.Get(ts.URL)
	a.Nil(err)

	body1, err := ioutil.ReadAll(res1.Body)
	defer res1.Body.Close()
	a.Nil(err)

	// ws
	res2 := wsGet(ts.URL, t)

	body2, err := ioutil.ReadAll(res2.Body)
	defer res2.Body.Close()
	a.Nil(err)

	a.Equal(string(body1), "false")
	a.Equal(string(body2), "true")
}

func wsGet(u string, t *testing.T) *http.Response {
	a := A{t}

	c := &http.Client{}

	req, err := http.NewRequest("GET", u, nil)
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Connection", "Upgrade")
	a.Nil(err)
	res, err := c.Do(req)
	a.Nil(err)

	return res
}
