package checks

import (
	"strings"
	"time"

	"github.com/fluffle/goirc/client"
)

type Irc struct {
	checkBase
	Command     []string
	Channel     string
	Stringmatch string
	Nick        string
	Msgcode     string
}

func (c Irc) Run(teamID uint, boxIp string, res chan Result) {

	irc := client.SimpleClient(c.Nick)

	// Add a handler that waits for the "disconnected" event and
	disconnected := make(chan struct{})
	irc.HandleFunc("disconnected", func(c *client.Conn, l *client.Line) {
		// closes a channel to signal everything is done.
		close(disconnected)
	})

	// Connect to an IRC server.
	if err := irc.ConnectTo(boxIp); err != nil {
		res <- Result{
			Error: "IRC connection failed",
			Debug: "IRC connection failed: " + err.Error(),
		}
		return
	}
	defer irc.Quit()

	irc.Join(c.Channel)

	didwork := make(chan bool)
	irc.HandleFunc(c.Msgcode, func(cconn *client.Conn, l *client.Line) {
		for _, arg := range l.Args {
			if strings.Contains(arg, c.Stringmatch) {
				close(didwork) // Close the channel if there's a match
				return
			}
		}
		didwork <- true
	})

	// raw command
	for _, cmd := range c.Command {
		irc.Raw(cmd)
	}

	select {
	case _, notok := <-didwork:
		if notok {
			res <- Result{
				Status: false,
				Debug:  "Bad response on IRC",
			}
		}
	case <-time.After(5 * time.Second):
		res <- Result{
			Status: false,
			Debug:  "Bad response on IRC",
		}
		return
	}

	res <- Result{
		Status: true,
		Debug:  "IRC check success",
	}

}
