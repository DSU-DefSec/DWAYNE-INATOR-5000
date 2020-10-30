package checks

import (
	"strconv"
)

type Smb struct {
	checkBase
	Port   int
	Domain string
	// ??
}

func (c Smb) Run(boxIp string, res chan Result) {
	// Authenticated SMB
	if len(c.CredLists) > 0 {
		username, password, _ := getCreds(c.CredLists)
		// log in smb
		// if err != nil {
		// return bad result
		// check if file is specified
		// retrieve file
		// check if hash is specified
		// compare hash

		res <- Result{
			Status: true,
			Debug:  "placeholder tcp. creds used were " + username + ":" + password,
		}
		return
	} else {
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
	}
	res <- Result{
		Status: true,
		Debug:  "anonymous smb connected",
	}
	// anonymous smb
}
