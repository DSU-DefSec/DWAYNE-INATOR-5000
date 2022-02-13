package checks

import (
	"fmt"
	"strings"

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
	scheme := "ldap"
	if c.Encrypted {
		scheme = "ldaps"
	}
	lconn, err := ldap.DialURL(fmt.Sprintf("%s://%s:%d", scheme, boxIp, c.Port))
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

	// Attempt to login
	splitDomain := strings.Split(c.Domain, ".")
	if len(splitDomain) != 2 {
		res <- Result{
			Error: "configured domain is not valid (needs to be domain and tld)",
		}
		return

	}

	authString := fmt.Sprintf("%s@%s", username, c.Domain)
	err = lconn.Bind(authString, password)
	if err != nil {
		res <- Result{
			Error: "login failed for " + username,
			Debug: "auth string " + authString + ", login " + username + " password " + password + " failed with error: " + err.Error(),
		}
		return
	}

	res <- Result{
		Status: true,
		Debug:  "login successful for username " + username + " password " + password,
	}
}
