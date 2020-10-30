package checks

import (
	"strconv"

	"github.com/miekg/dns"
)

type Dns struct {
	checkBase
	Port   int
	Domain string
	// ??
}

func (c Dns) Run(teamName, boxIp string, res chan Result) {
	println(new(dns.Client))
	/*
		client := new(dns.Client)
		laddr := net.UDPAddr{
			IP: net.ParseIP("[::1]"),
			Port: 12345,
			Zone: "",
		}
		client.Dialer := &net.Dialer{
			Timeout: 200 * time.Millisecond,
			LocalAddr: &laddr,
		}
		in, rtt, err := c.Exchange(m1, "8.8.8.8:53")
	*/

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
