package checks

import (
	"strconv"

	"github.com/icodeface/grdp"
)

type Rdp struct {
	checkBase
	Port int
	// ??
}

func (c Rdp) Run(boxIp string, res chan Result) {
	if len(c.CredLists) == 0 {
		// PLACEHOLDER: test tcp only
		err := tcpCheck(boxIp + ":" + strconv.Itoa(c.Port))
		if err != nil {
			res <- Result{
				Status: false,
				Error:  "connection error",
				Debug:  err.Error(),
			}
			return
		}
		res <- Result{
			Status: true,
		}
		return
	}

	username, password, err := getCreds(c.CredLists)
	if err != nil {
		res <- Result{
			Status: false,
			Error:  "no credlists supplied to check",
		}
		return
	}
	client := grdp.NewClient(boxIp, 0)
	err = client.Login(username, password)
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
