package checks

import (
	"strconv"

	"github.com/icodeface/grdp"
)

type Sql struct {
	checkBase
	Port int
	// ??
}

func (c Sql) Run(teamName, boxIp string, res chan Result) {
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

	username, password, err := getCreds(c.CredLists, teamName, c.Name)
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
