package checks

import (
	"strconv"
	"math/rand"
	"io/ioutil"
	"regexp"
	"time"

	"github.com/jlaffaye/ftp"
)

type Ftp struct {
	checkBase
	Port        int
	File       []FtpFile
	BadAttempts int
}

type FtpFile struct {
	Name string
	Hash string
	Regex string
}

func (c Ftp) Run(teamName, boxIp string, res chan Result) {
	conn, err := ftp.Dial(boxIp+":"+strconv.Itoa(c.Port), ftp.DialWithTimeout(time.Duration(GlobalTimeout)*time.Second))
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
		username, password, err = getCreds(c.CredLists, teamName, c.Name)
		if err != nil {
			res <- Result{
				Status: false,
				Error:  "no credlists supplied to check",
			}
			return
		}
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
	defer conn.Quit()


	if len(c.File) > 0 {
		file := c.File[rand.Intn(len(c.File))]
		r, err := conn.Retr(file.Name)
		if err != nil {
			res <- Result{
				Status: true,
				Error: "ftp login suceeded",
				Debug: "creds used were " + username + ":" + password,
			}
			return
		}
		defer r.Close()
		buf, err := ioutil.ReadAll(r)
		if err != nil {
			res <- Result{
				Error:  "failed to read ftp file",
				Debug:  "tried to read " + file.Name,
			}
			return
		}
		if file.Regex != "" {
			re, err := regexp.Compile(file.Regex)
			if err != nil {
				res <- Result{
					Error:  "error compiling regex to match for ftp file",
					Debug:  err.Error(),
				}
				return
			}
			reFind := re.Find(buf)
			if reFind == nil {
				res <- Result{
					Error: "couldn't find regex in file",
					Debug: "couldn't find regex \"" + file.Regex + "\" for " + file.Name,
				}
				return
			} else {
				res <- Result{
					Status: true,
					Error:  "file matched regex",
					Debug:  "matched regex " + file.Regex + " for " + file.Name,
				}
				return
			}
		} else {
			// todo hash :pensive:
		}
	} else {
		res <- Result{
			Status: true,
			Error: "ftp login suceeded",
			Debug: "creds used were " + username + ":" + password,
		}
		return
	}
}
