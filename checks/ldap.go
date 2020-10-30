package checks

import (
	"strconv"
)

type Ldap struct {
	checkBase
	Port   int
	Domain string
	// ??
}

func (c Ldap) Run(boxIp string, res chan Result) {
	// execute commands

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
		Debug:  "check ran",
	}
}
