package checks

import (
	"os/exec"
	"regexp"
	"strings"
)

type Cmd struct {
	checkBase
	Command string
	Regex   string
}

func commandOutput(cmd string) (string, error) {
	out, err := exec.Command("/bin/sh", "-c", cmd).Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func (c Cmd) Run(teamID uint, boxIp string, res chan Result) {
	re, err := regexp.Compile(c.Regex)
	if err != nil {
		res <- Result{
			Error: "error compiling regex",
			Debug: err.Error(),
		}
		return
	}

	out, err := commandOutput(strings.Replace(c.Command, "BOXIP", boxIp, -1))
	if err != nil {
		res <- Result{
			Error: "command returned error",
			Debug: err.Error(),
		}
		return
	}

	reFind := re.Find([]byte(out))
	if reFind == nil {
		res <- Result{
			Error: "output incorrect",
			Debug: "couldn't find regex \"" + c.Regex + "\" in " + out,
		}
		return
	}

	res <- Result{
		Status: true,
	}
}
