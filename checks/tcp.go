package checks

import (
	"strconv"
)

type Tcp struct {
	checkBase
}

func (c Tcp) Run(teamID uint, boxIp string, res chan Result) {
	err := tcpCheck(boxIp + ":" + strconv.Itoa(c.Port))
	if err != nil {
		res <- Result{
			Error: "connection error",
			Debug: err.Error(),
		}
		return
	}
	res <- Result{
		Status: true,
		Debug:  "responded to request",
	}
}
