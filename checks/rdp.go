package checks

import (
	"strconv"
	// why are there no good rdp libraries?
)

type Rdp struct {
	checkBase
}

func (c Rdp) Run(teamID uint, boxIp string, res chan Result) {
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
		Debug:  "responded",
	}
}
