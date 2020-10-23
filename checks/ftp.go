package checks

import (
	"strconv"
	"time"

	"github.com/jlaffaye/ftp"
)

type Ftp struct {
	checkBase
	Anonymous   bool
	Port        int
	Timeout     int
	Files       []string
	Hashes      []string
	BadAttempts int
}

func (c Ftp) Run(teamPrefix string, res chan Result) {
	// harcoded 5 second timeout
	conn, err := ftp.Dial(teamPrefix+c.Suffix+":"+strconv.Itoa(c.Port), ftp.DialWithTimeout(time.Duration(c.Timeout)*time.Second))
	if err != nil {
		res <- Result{
			Status: false,
			Error:  "ftp connection failed",
			Debug:  err.Error(),
		}
		return
	}

	var username, password string
	if c.Anonymous {
		username = "anonymous"
		password = "anonymous"
	} else {
		username, password = getCreds(c.CredLists)
	}
	err = conn.Login(username, password)
	if err != nil {
		res <- Result{
			Status: false,
			Error:  "ftp login failed",
			Debug:  "creds used were " + username + ":" + password + " with error " + err.Error(),
		}
		return
	}

	// Do something with the FTP conn
	if err := conn.Quit(); err != nil {
		res <- Result{
			Status: false,
			Error:  "quitting FTP session failed",
			Debug:  err.Error(),
		}
		return
	}

	res <- Result{
		Status: true,
	}
}
