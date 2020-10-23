package main

import (
	"errors"
	"io/ioutil"
	"log"
	"sort"
	"strconv"
	"time"

	"github.com/DSU-DefSec/mew/checks"

	"github.com/BurntSushi/toml"
)

type config struct {
	Event        string
	Kind         string
	Verbose      bool
	Tightlipped  bool
	Delay        int
	Jitter       int
	Timeout      int
	SlaThreshold int
	SlaPoints    int
	Admin        []adminData
	Team         []teamData
	CheckList    []checks.Check
	Creds        []checks.CredData
	AllChecks
}

type AllChecks struct {
	Dns []checks.Dns
	Ftp []checks.Ftp
	Ldap []checks.Ldap
	Rdp []checks.Rdp
	Smb []checks.Smb
	Ssh []checks.Ssh
	Web []checks.Web
}

const (
	configPath = "./mew.conf"
)

func fillCheckList(m *config) {
	// Gotta be a better way to do this
	for _, c := range m.Dns {
		m.CheckList = append(m.CheckList, c)
	}
	for _, c := range m.Ftp {
		m.CheckList = append(m.CheckList, c)
	}
	for _, c := range m.Rdp {
		m.CheckList = append(m.CheckList, c)
	}
	for _, c := range m.Smb {
		m.CheckList = append(m.CheckList, c)
	}
	for _, c := range m.Ssh {
		m.CheckList = append(m.CheckList, c)
	}
	for _, c := range m.Web {
		m.CheckList = append(m.CheckList, c)
	}
}

func readConfig(conf *config) {
	fileContent, err := ioutil.ReadFile(configPath)
	if err != nil {
		log.Fatalln("Configuration file ("+configPath+") not found:", err)
	}
	if _, err := toml.Decode(string(fileContent), &conf); err != nil {
		log.Fatalln(err)
	}
}

func checkConfig(conf *config) error {
	// general error checking
	if conf.Event == "" {
		return errors.New("event title blank or not specified")
	}

	if conf.Kind == "" {
		conf.Kind = "dcdc"
	}

	if conf.Delay == 0 {
		conf.Delay = defaultDelay
	}

	if conf.Jitter == 0 {
		conf.Jitter = 30
	}

	if conf.Jitter >= conf.Delay {
		return errors.New("illegal config: jitter not smaller than delay")
	}

	if conf.Timeout >= conf.Delay - conf.Jitter {
		return errors.New("illegal config: timeout not smaller than delay minus jitter")
	}

	if conf.Timeout != 0 {
		dur, err := time.ParseDuration(strconv.Itoa(conf.Timeout) + "s")
		if err != nil {
			return errors.New("illegal config: invalid value for timeout: " + err.Error())
		}
		checks.GlobalTimeout = dur
	}

	for _, admin := range conf.Admin {
		if admin.Name == "" || admin.Pw == "" {
			return errors.New("admin" + admin.Name + "missing required property")
		}
	}

	// setting defaults

	// If Tightlipped is enabled, Verbose can not be enabled.
	if conf.Tightlipped && conf.Verbose {
		conf.Verbose = false
	}

	if conf.SlaThreshold == 0 {
		conf.SlaThreshold = 6
	}

	if conf.SlaPoints == 0 {
		conf.SlaPoints = conf.SlaThreshold * 2
	}

	// credential list checking
	usernameList := []string{}
	for _, c := range conf.Creds {
		// set checks.CredLists and default passwords
		usernameList = append(usernameList, c.Usernames...)
		checks.CredLists[c.Name] = c.Usernames
		for _, u := range c.Usernames {
			checks.Creds[u] = c.DefaultPw
		}
	}

	// sort creds and look for duplicate usernames
	sort.SliceStable(usernameList, func(i, j int) bool {
		return usernameList[i] < usernameList[j]
	})

	for i := 0; i < len(usernameList)-1; i++ {
		if usernameList[i] == usernameList[i+1] {
			return errors.New("illegal config: duplicate username found in cred lists: " + usernameList[i])
		}
	}

	// look for duplicate team names
	sort.SliceStable(conf.Team, func(i, j int) bool {
		return conf.Team[i].Name < conf.Team[i].Name
	})

	for i := 0; i < len(conf.Team)-1; i++ {
		if conf.Team[i].Name == conf.Team[i+1].Name {
			return errors.New("illegal config: duplicate team name found")
		}
	}

	// look for missing team properties
	for _, team := range conf.Team {
		if team.Name == "" || team.Pw == "" || team.Prefix == "" {
			return errors.New("team " + team.Name + "missing required property")
		}
	}

	// sort CheckList
	sort.SliceStable(conf.CheckList, func(i, j int) bool {
		return conf.CheckList[i].FetchName() > conf.CheckList[j].FetchName()
	})

	// check validators
	for i, c := range conf.CheckList {
		switch c.(type) {
		case checks.Ftp:
			ck := c.(checks.Ftp)
			if ck.Port == 0 {
				ck.Port = 21
			}
			if ck.Timeout == 0 {
				ck.Timeout = 5
			}
			conf.CheckList[i] = ck
		case checks.Rdp:
			ck := c.(checks.Rdp)
			if ck.Port == 0 {
				ck.Port = 3389
				conf.CheckList[i] = ck
			}
		case checks.Smb:
			ck := c.(checks.Smb)
			if ck.Port == 0 {
				ck.Port = 445
				conf.CheckList[i] = ck
			}
		case checks.Ssh:
			ck := c.(checks.Ssh)
			if ck.Port == 0 {
				ck.Port = 22
				conf.CheckList[i] = ck
			}
		case checks.Web:
			ck := c.(checks.Web)
			if len(ck.Url) == 0 {
				return errors.New("no urls specified for web check")
			}
			for j, u := range ck.Url {
				if u.Scheme == "" {
					ck.Url[j].Scheme = "http"
				}
				if u.Port == 0 {
					ck.Url[j].Port = 80
				}
			}
			conf.CheckList[i] = ck
		}
	}

	// look for duplicate checks
	for i := 0; i < len(conf.CheckList)-1; i++ {
		if conf.CheckList[i].FetchName() == conf.CheckList[i+1].FetchName() {
			return errors.New("duplicate check name found")
		}
	}

	return nil
}

func sortCheckList(checkList []checks.Check) []checks.Check {
	return checkList
}
