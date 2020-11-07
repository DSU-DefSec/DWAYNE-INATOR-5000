package checks

import (
	"strconv"
	"github.com/stacktitan/smb/smb"
)

type Smb struct {
	checkBase
	Port   int
	Domain string
	// ??
}

func (c Smb) Run(teamName, boxIp string, res chan Result) {
	// Authenticated SMB
	if !c.Anonymous {
		username, password, _ := getCreds(c.CredLists, teamName, c.Name)
		options := smb.Options{
			Host:        boxIp,
			Port:        445,
			User:        username,
			Domain:      c.Domain,
			Workstation: "",
			Password:    password,
		}
		session, err := smb.NewSession(options, false)
		if err != nil {
			res <- Result{
				Error:  "smb session creation failed",
				Debug:  "creds " + username + ":" + password,
			}
			return
		}
		defer session.Close()

		if session.IsAuthenticated {
			res <- Result{
				Status: true,
				Error:  "smb login succeeded",
				Debug:  "creds " + username + ":" + password,
			}
			return
		} else {
			res <- Result{
				Error:  "smb login failed",
				Debug:  "creds " + username + ":" + password,
			}
			return
		}
	} else {
		// PLACEHOLDER: test tcp only
		err := tcpCheck(boxIp + ":" + strconv.Itoa(c.Port))
		if err != nil {
			res <- Result{
				Error:  "connection error",
				Debug:  err.Error(),
			}
			return
		}
	}
	res <- Result{
		Status: true,
		Debug:  "responded to tcp request",
	}
	// anonymous smb
}
