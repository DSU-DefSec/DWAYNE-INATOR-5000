package checks

import (
	"crypto/tls"
	"fmt"

	ldap "github.com/go-ldap/ldap/v3"
)

type Ldap struct {
	checkBase
	Domain    string
	Encrypted bool
}

func (c Ldap) Run(teamID uint, boxIp string, res chan Result) {
	// Set timeout
	ldap.DefaultTimeout = GlobalTimeout

	username, password := getCreds(teamID, c.CredList, c.Name)
	// Normal, default ldap check
	lconn, err := ldap.Dial("tcp", fmt.Sprintf("%s:%d", boxIp, c.Port))
	if err != nil {
		res <- Result{
			Error: "failed to connect",
			Debug: "login " + username + " password " + password + " failed with error: " + err.Error(),
		}
		return
	}
	defer lconn.Close()

	// Set message timeout
	lconn.SetTimeout(GlobalTimeout)

	// Add TLS if needed
	if c.Encrypted {
		err = lconn.StartTLS(&tls.Config{InsecureSkipVerify: true})
		if err != nil {
			res <- Result{
				Error: "tls session creation failed",
				Debug: "login " + username + " password " + password + " failed with error: " + err.Error(),
			}
			return
		}
	}

	// Attempt to login
	err = lconn.Bind(username, password)
	if err != nil {
		res <- Result{
			Error: "login failed for " + username,
			Debug: "login " + username + " password " + password + " failed with error: " + err.Error(),
		}
		return
	}

	res <- Result{
		Status: true,
		Debug:  "login successful for username " + username + " password " + password,
	}
}
