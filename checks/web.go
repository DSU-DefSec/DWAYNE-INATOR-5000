package checks

import (
	"io/ioutil"
	"net/http"
	"strconv"
	"regexp"
)

type Web struct {
	checkBase
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
	Regex		string
}

func (c Web) Run(teamName, boxIp string, res chan Result) {
	for i, u := range c.Url {
		// if usernameParam == nil
		// post with username/pw as creds
		// else
        tr := &http.Transport{
            MaxIdleConns:       1,
            IdleConnTimeout:    GlobalTimeout,
            DisableKeepAlives: true,
        }
        client := &http.Client{Transport: tr}
		resp, err := client.Get(u.Scheme + "://" + boxIp + ":" + strconv.Itoa(c.Port) + u.Path)
		if err != nil {
			res <- Result{
				Error:  "web request errored out",
				Debug:  err.Error() + " for url " + strconv.Itoa(i),
			}
			return
		}

		if u.Status != 0 && resp.StatusCode != u.Status {
			res <- Result{
				Error:  "status returned by webserver was incorrect",
				Debug:  "status was " + strconv.Itoa(resp.StatusCode) + " wanted " + strconv.Itoa(u.Status) + " for url " + strconv.Itoa(i),
			}
			return
		}

		defer resp.Body.Close()
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			res <- Result{
				Error:  "error reading page content",
				Debug:  "error was '" + err.Error() + "' for url " + strconv.Itoa(i),
			}
			return
		}

		if u.Regex != "" {
			re, err := regexp.Compile(u.Regex)
			if err != nil {
				res <- Result{
					Error:  "error compiling regex to match for web page",
					Debug:  err.Error(),
				}
				return
			}
			reFind := re.Find(body)
			if reFind == nil {
				res <- Result{
					Error:  "didn't find regex on page :(",
					Debug:  "couldn't find regex \"" + u.Regex + "\" for " + u.Path,
				}
				return
			} else {
				res <- Result{
					Status: true,
					Error:  "page matched regex!",
					Debug:  "matched regex \"" + u.Regex + "\" for " + u.Path,
				}
				return
			}

		}
	}

	res <- Result{
		Status: true,
	}
}
