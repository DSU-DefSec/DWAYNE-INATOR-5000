package checks

import (
	"math/rand"
	"regexp"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"golang.org/x/crypto/ssh"
)

type Ssh struct {
	checkBase
	PubKey      string
	BadAttempts int
	Command     []commandData
}

type commandData struct {
	UseRegex bool
	Contains bool
	Command  string
	Output   string
}

func (c Ssh) Run(teamID uint, boxIp string, res chan Result) {
	// if  pubkey
	// var hostKey ssh.PublicKey
	// pubkey
	// else
	username, password := getCreds(teamID, c.CredList, c.Name)

	// Create client config
	config := &ssh.ClientConfig{
		User: username,
		Auth: []ssh.AuthMethod{
			ssh.Password(password),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         GlobalTimeout,
	}

	for i := 0; i < c.BadAttempts; i++ {
		badConf := &ssh.ClientConfig{
			User: username,
			Auth: []ssh.AuthMethod{
				ssh.Password(uuid.New().String()),
			},
			HostKeyCallback: ssh.InsecureIgnoreHostKey(),
			Timeout:         GlobalTimeout,
		}

		badConn, err := ssh.Dial("tcp", boxIp+":"+strconv.Itoa(c.Port), badConf)
		if err == nil {
			badConn.Close()
		}
	}

	// Connect to ssh server
	conn, err := ssh.Dial("tcp", boxIp+":"+strconv.Itoa(c.Port), config)
	if err != nil {
		res <- Result{
			Error: "error logging in to ssh server for creds " + username + ":" + password,
			Debug: "error: " + err.Error(),
		}
		return
	}
	defer conn.Close()

	// Create a session
	session, err := conn.NewSession()
	if err != nil {
		res <- Result{
			Error: "unable to create ssh session",
			Debug: err.Error(),
		}
		return
	}
	defer session.Close()

	// Set up terminal modes
	modes := ssh.TerminalModes{
		ssh.ECHO:          0,     // disable echoing
		ssh.TTY_OP_ISPEED: 14400, // input speed = 14.4kbaud
		ssh.TTY_OP_OSPEED: 14400, // output speed = 14.4kbaud
	}

	// Request pseudo terminal
	if err := session.RequestPty("xterm", 40, 80, modes); err != nil {
		res <- Result{
			Error: "couldn't allocate pts",
			Debug: err.Error(),
		}
		return
	}

	// Start remote shell
	if err := session.Shell(); err != nil {
		res <- Result{
			Error: "failed to start shell",
			Debug: "error: " + err.Error(),
		}
		return
	}

	// If any commands specified, run them
	if len(c.Command) > 0 {
		r := c.Command[rand.Intn(len(c.Command))]
		output, err := session.CombinedOutput(r.Command)
		if err != nil {
			res <- Result{
				Error: "command execution failed",
				Debug: err.Error(),
			}
			return
		}
		if r.Output != "" {
			if r.Contains {
				if !strings.Contains(string(output), r.Output) {
					res <- Result{
						Error: "command output didn't contain string",
						Debug: "command output of '" + r.Command + "' didn't contain string '" + r.Output,
					}
					return
				}
			} else if r.UseRegex {
				re := regexp.MustCompile(r.Output)
				if !re.Match(output) {
					res <- Result{
						Error: "command output didn't match regex",
						Debug: "command output'" + r.Command + "' didn't match regex '" + r.Output,
					}
					return
				} else {
					if strings.TrimSpace(string(output)) != r.Output {
						res <- Result{
							Error: "command output didn't match string",
							Debug: "command output of '" + r.Command + "' didn't match string '" + r.Output,
						}
						return
					}
				}
			}
		}
	}
	res <- Result{
		Status: true,
		Debug:  "creds used were " + username + ":" + password,
	}
}
