package main

import (
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/DSU-DefSec/mew/checks"
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
	Box          []Box
	Creds        []checks.CredData
}

type Box struct {
	Name      string
	Suffix    string
	CheckList []checks.Check
	Dns       []checks.Dns
	Ftp       []checks.Ftp
	Ldap      []checks.Ldap
	Rdp       []checks.Rdp
	Smb       []checks.Smb
	Ssh       []checks.Ssh
	Web       []checks.Web
}

const (
	configPath = "./mew.conf"
)

func getBoxChecks(b Box) []checks.Check {
	// Gotta be a better way to do this
	checkList := []checks.Check{}
	for _, c := range b.Dns {
		checkList = append(checkList, c)
	}
	for _, c := range b.Ftp {
		checkList = append(checkList, c)
	}
	for _, c := range b.Ldap {
		checkList = append(checkList, c)
	}
	for _, c := range b.Rdp {
		checkList = append(checkList, c)
	}
	for _, c := range b.Smb {
		checkList = append(checkList, c)
	}
	for _, c := range b.Ssh {
		checkList = append(checkList, c)
	}
	for _, c := range b.Web {
		checkList = append(checkList, c)
	}
	return checkList
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

	if conf.Timeout >= conf.Delay-conf.Jitter {
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

	// apply default cred lists
	if len(mewConf.Creds) > 0 {
		checks.DefaultCredList = mewConf.Creds[0].Usernames
	}

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
			checks.DefaultCreds[u] = c.DefaultPw
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
			return errors.New("team " + team.Name + " missing required property, one of name, password, or prefix")
		}
	}

	// check validators
	for i, b := range conf.Box {
		conf.Box[i].CheckList = getBoxChecks(b)
		for j, c := range conf.Box[i].CheckList {
			switch c.(type) {
			case checks.Dns:
				ck := c.(checks.Dns)
				ck.Suffix = b.Suffix
				if ck.Name == "" {
					ck.Name = b.Name + "-" + "dns"
				}
				if ck.Display == "" {
					ck.Display = "dns"
				}
				if ck.Port == 0 {
					ck.Port = 53
				}
				conf.Box[i].CheckList[j] = ck
			case checks.Ftp:
				ck := c.(checks.Ftp)
				ck.Suffix = b.Suffix
				if ck.Name == "" {
					ck.Name = b.Name + "-" + "ftp"
				}
				if ck.Display == "" {
					ck.Display = "ftp"
				}
				if ck.Port == 0 {
					ck.Port = 21
				}
				if ck.Timeout == 0 {
					ck.Timeout = 5
				}
				conf.Box[i].CheckList[j] = ck
			case checks.Ldap:
				ck := c.(checks.Ldap)
				ck.Suffix = b.Suffix
				if ck.Name == "" {
					ck.Name = b.Name + "-" + "ldap"
				}
				if ck.Display == "" {
					ck.Display = "ldap"
				}
				if ck.Port == 0 {
					ck.Port = 636
				}
				conf.Box[i].CheckList[j] = ck
			case checks.Rdp:
				ck := c.(checks.Rdp)
				if ck.Name == "" {
					ck.Name = b.Name + "-" + "rdp"
				}
				if ck.Display == "" {
					ck.Display = "rdp"
				}
				ck.Suffix = b.Suffix
				if ck.Port == 0 {
					ck.Port = 3389
				}
				conf.Box[i].CheckList[j] = ck
			case checks.Smb:
				ck := c.(checks.Smb)
				ck.Suffix = b.Suffix
				if ck.Name == "" {
					ck.Name = b.Name + "-" + "smb"
				}
				if ck.Display == "" {
					ck.Display = "smb"
				}
				if ck.Port == 0 {
					ck.Port = 445
				}
				conf.Box[i].CheckList[j] = ck
			case checks.Ssh:
				ck := c.(checks.Ssh)
				ck.Suffix = b.Suffix
				if ck.Name == "" {
					ck.Name = b.Name + "-" + "ssh"
				}
				if ck.Display == "" {
					ck.Display = "ssh"
				}
				if ck.Port == 0 {
					ck.Port = 22
				}
				conf.Box[i].CheckList[j] = ck
			case checks.Web:
				ck := c.(checks.Web)
				ck.Suffix = b.Suffix
				if ck.Name == "" {
					ck.Name = b.Name + "-" + "web"
				}
				if ck.Display == "" {
					ck.Display = "web"
				}
				if ck.Port == 0 {
					ck.Port = 80
				}
				if len(ck.Url) == 0 {
					return errors.New("no urls specified for web check")
				}
				for j, u := range ck.Url {
					if u.Scheme == "" {
						ck.Url[j].Scheme = "http"
					}
				}
				conf.Box[i].CheckList[j] = ck
			}
		}

	}

	// look for duplicate checks
	for _, b := range conf.Box {
		for j := 0; j < len(b.CheckList)-1; j++ {
			if b.CheckList[j].FetchName() == b.CheckList[j+1].FetchName() {
				return errors.New("duplicate check name '" + b.CheckList[j].FetchName() + "' and '" + b.CheckList[j+1].FetchName() + "' for box " + b.Name)
			}
		}
	}

	return nil
}

func getCheckName(check checks.Check) string {
	name := strings.Split(reflect.TypeOf(check).String(), ".")[1]
	fmt.Println("name is ", name)
	return name
}
