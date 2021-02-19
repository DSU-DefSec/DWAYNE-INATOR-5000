package checks

import (
	"io/ioutil"
	"math/rand"
	"regexp"
	"strconv"
	"time"

	"github.com/jlaffaye/ftp"
)

type Ftp struct {
	checkBase
	File []FtpFile
}

type FtpFile struct {
	Name  string
	Hash  string
	Regex string
}

func (c Ftp) Run(teamName, boxIp string, res chan Result) {
	conn, err := ftp.Dial(boxIp+":"+strconv.Itoa(c.Port), ftp.DialWithTimeout(time.Duration(GlobalTimeout)*time.Second))
	if err != nil {
		res <- Result{
			Error: "ftp connection failed",
			Debug: err.Error(),
		}
		return
	}

	var username, password string
	if c.Anonymous {
		username = "anonymous"
		password = "anonymous"
	} else {
		username, password = getCreds(c.CredLists, teamName, c.Name)
	}
	err = conn.Login(username, password)
	if err != nil {
		res <- Result{
			Error: "ftp login failed",
			Debug: "creds used were " + username + ":" + password + " with error " + err.Error(),
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
				Error:  "ftp login suceeded",
				Debug:  "creds used were " + username + ":" + password,
			}
			return
		}
		defer r.Close()
		buf, err := ioutil.ReadAll(r)
		if err != nil {
			res <- Result{
				Error: "failed to read ftp file",
				Debug: "tried to read " + file.Name,
			}
			return
		}
		if file.Regex != "" {
			re, err := regexp.Compile(file.Regex)
			if err != nil {
				res <- Result{
					Error: "error compiling regex to match for ftp file",
					Debug: err.Error(),
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
			}
		} else if file.Hash != "" {
			fileHash, err := StringHash(string(buf))
			if err != nil {
				res <- Result{
					Error: "error calculating file hash",
					Debug: err.Error(),
				}
				return
			} else if fileHash != file.Hash {
				res <- Result{
					Error: "file hash did not match",
					Debug: "file hash " + fileHash + " did not match specified hash " + file.Hash,
				}
				return
			}
		}
	}
	res <- Result{
		Status: true,
		Debug:  "creds used were " + username + ":" + password,
	}
}
