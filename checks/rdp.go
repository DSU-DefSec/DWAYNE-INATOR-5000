package checks

import (
	"net"
	"strconv"
	"time"

	"github.com/icodeface/grdp"
)

type Rdp struct {
	checkBase
	Port int
	// ??
}

func (c Rdp) Run(teamPrefix string, res chan Result) {
	host := teamPrefix + c.Suffix + ":" + strconv.Itoa(c.Port)

	if len(c.CredLists) == 0 {
		// PLACEHOLDER: test tcp only
		conn, err := net.DialTimeout("tcp", host, 3*time.Second)
		if err != nil {
			res <- Result{
				Status: false,
				Error:  "connection error",
				Debug:  err.Error(),
			}
			return
		}
		defer conn.Close()
		res <- Result{
			Status: true,
		}
		return
	}

	username, password := getCreds(c.CredLists)
	client := grdp.NewClient(host, 0)
	err := client.Login(username, password)
	if err != nil {
		res <- Result{
			Status: false,
			Error:  "rdp connection or login failed",
			Debug:  err.Error() + ", creds used were " + username + ":" + password,
		}
		return
	}
	res <- Result{
		Status: true,
		Debug:  "creds used were " + username + ":" + password,
	}
}
