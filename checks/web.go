package checks

import (
	"io/ioutil"
	"net/http"
	"strconv"
	"time"
)

type Web struct {
	checkBase
	Timeout int
	Port    int
	Url     []urlData
}

type urlData struct {
	Scheme string
	Path   string
	// use creds list for check for login
	UsernameParam string
	PasswordParam string
	Status        int
	Diff          int
}

func (c Web) Run(teamName, boxIp string, res chan Result) {
	timeout := c.Timeout
	if timeout == 0 {
		timeout = 10
	}

	client := http.Client{
		Timeout: time.Duration(timeout) * time.Second,
	}

	for i, u := range c.Url {
		// if usernameParam == nil
		// post with username/pw as creds
		// else
		resp, err := client.Get(u.Scheme + "://" + boxIp + ":" + strconv.Itoa(c.Port) + u.Path)
		if err != nil {
			res <- Result{
				Status: false,
				Error:  "web request errored out",
				Debug:  err.Error() + " for url " + strconv.Itoa(i),
			}
			return
		}

		if u.Status != 0 && resp.StatusCode != u.Status {
			res <- Result{
				Status: false,
				Error:  "status returned by webserver was incorrect",
				Debug:  "status was " + strconv.Itoa(resp.StatusCode) + " wanted " + strconv.Itoa(u.Status) + " for url " + strconv.Itoa(i),
			}
			return
		}

		defer resp.Body.Close()
		_, err = ioutil.ReadAll(resp.Body)
		// fmt.Println("body", body)
	}

	res <- Result{
		Status: true,
	}
}
