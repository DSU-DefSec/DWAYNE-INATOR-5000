package main

import (
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/DSU-DefSec/DWAYNE-INATOR-5000/checks"
)

type config struct {
	Event       string
	Verbose     bool
	NoPasswords bool
	EasyPCR     bool
	// Score persistence or not (for purple team comps)
	Persists     bool
	Delay        int
	Jitter       int
	Timeout      int
	SlaThreshold int
	// Points per service check.
	ServicePoints int
	SlaPoints     int
	Admin         []TeamData
	Red           []TeamData
	Team          []TeamData
	Box           []Box
	Creds         []checks.CredData
	// Inject API key
	InjectAPIKey string
}

type Box struct {
	Name      string
	IP        string
	CheckList []checks.Check
	Cmd       []checks.Cmd
	Dns       []checks.Dns
	Ftp       []checks.Ftp
	Imap      []checks.Imap
	Ldap      []checks.Ldap
	Ping      []checks.Ping
	Rdp       []checks.Rdp
	Smb       []checks.Smb
	Smtp      []checks.Smtp
	Sql       []checks.Sql
	Ssh       []checks.Ssh
	Tcp       []checks.Tcp
	Vnc       []checks.Vnc
	Web       []checks.Web
	WinRM     []checks.WinRM
}

const (
	configPath = "./dwayne.conf"
)

func getBoxChecks(b Box) []checks.Check {
	// Please forgive me
	checkList := []checks.Check{}
	for _, c := range b.Cmd {
		checkList = append(checkList, c)
	}
	for _, c := range b.Dns {
		checkList = append(checkList, c)
	}
	for _, c := range b.Ftp {
		checkList = append(checkList, c)
	}
	for _, c := range b.Imap {
		checkList = append(checkList, c)
	}
	for _, c := range b.Ping {
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
	for _, c := range b.Smtp {
		checkList = append(checkList, c)
	}
	for _, c := range b.Sql {
		checkList = append(checkList, c)
	}
	for _, c := range b.Ssh {
		checkList = append(checkList, c)
	}
	for _, c := range b.Tcp {
		checkList = append(checkList, c)
	}
	for _, c := range b.Vnc {
		checkList = append(checkList, c)
	}
	for _, c := range b.Web {
		checkList = append(checkList, c)
	}
	for _, c := range b.WinRM {
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

	if conf.Delay == 0 {
		conf.Delay = defaultDelay
	}

	if conf.Jitter == 0 {
		conf.Jitter = 30
	}

	if conf.InjectAPIKey == "" {
		log.Println("WARNING: No Inject API Key specified, setting to random UUID")
		conf.InjectAPIKey = getUUID()
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
	} else {
		checks.GlobalTimeout = time.Second * 30
	}

	for _, admin := range conf.Admin {
		if admin.Name == "" || admin.Pw == "" {
			return errors.New("admin " + admin.Name + " missing required property")
		}
	}

	// setting defaults

	// apply default cred lists
	if len(dwConf.Creds) == 0 {
		return errors.New("illegal config: no valid credentials")
	}

	if conf.SlaThreshold == 0 {
		conf.SlaThreshold = 6
	}

	if conf.ServicePoints == 0 {
		conf.ServicePoints = 10
	}

	if conf.SlaPoints == 0 {
		conf.SlaPoints = conf.SlaThreshold * 2
	}

	// sort boxes
	sort.SliceStable(conf.Box, func(i, j int) bool {
		return conf.Box[i].IP < conf.Box[j].IP
	})

	// credential list checking
	usernameList := []string{}
	for _, c := range conf.Creds {
		usernameList = append(usernameList, c.Usernames...)
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

	// look for duplicate team prefix
	sort.SliceStable(conf.Team, func(i, j int) bool {
		return conf.Team[i].IP < conf.Team[i].IP
	})

	for i := 0; i < len(conf.Team)-1; i++ {
		if conf.Team[i].IP == "" {
			return errors.New("illegal config: non-set prefix for team")
		}
		if conf.Team[i].IP == conf.Team[i+1].IP {
			return errors.New("illegal config: duplicate team prefix found")
		}
	}

	// assign team identifiers
	for i := range conf.Team {
		if strings.TrimSpace(conf.Team[i].Name) == "" {
			conf.Team[i].Name = "team" + strconv.Itoa(i+1)
		}
	}

	// look for missing team properties
	for _, team := range conf.Team {
		if team.Name == "" || team.Pw == "" || team.IP == "" {
			return errors.New("illegal config: team missing one or more required property: name, password, or prefix")
		}
	}

	// if persists, make sure they have tokens
	if conf.Persists {
		for _, team := range conf.Team {
			if team.Token == "" {
				return errors.New("illegal config: team " + team.Name + " missing persist token")
			}
		}

		// look for duplicate token
		sort.SliceStable(conf.Team, func(i, j int) bool {
			return conf.Team[i].Token < conf.Team[i].Token
		})

		for i := 0; i < len(conf.Team)-1; i++ {
			if conf.Team[i].Token == conf.Team[i+1].Token {
				return errors.New("illegal config: duplicate team persist tokens found: " + conf.Team[i].Token)
			}
		}

		// sort by ip again lol
		sort.SliceStable(conf.Team, func(i, j int) bool {
			return conf.Team[i].IP < conf.Team[i].IP
		})
	}

	// check validators
	// please overlook this transgression
	for i, b := range conf.Box {
		conf.Box[i].CheckList = getBoxChecks(b)
		for j, c := range conf.Box[i].CheckList {
			switch c.(type) {
			case checks.Cmd:
				ck := c.(checks.Cmd)
				ck.IP = b.IP
				if ck.Display == "" {
					ck.Display = "cmd"
				}
				if ck.Name == "" {
					ck.Name = b.Name + "-" + ck.Display
				}
				if ck.CredList == "" {
					ck.Anonymous = true
				}
				conf.Box[i].CheckList[j] = ck
			case checks.Dns:
				ck := c.(checks.Dns)
				ck.IP = b.IP
				ck.Anonymous = true // call me when you need authed DNS
				if ck.Display == "" {
					ck.Display = "dns"
				}
				if ck.Name == "" {
					ck.Name = b.Name + "-" + ck.Display
				}
				if len(ck.Record) < 1 {
					return errors.New("dns check " + ck.Name + " has no records")
				}
				if ck.Port == 0 {
					ck.Port = 53
				}
				conf.Box[i].CheckList[j] = ck
			case checks.Ftp:
				ck := c.(checks.Ftp)
				ck.IP = b.IP
				if ck.Display == "" {
					ck.Display = "ftp"
				}
				if ck.Name == "" {
					ck.Name = b.Name + "-" + ck.Display
				}
				if ck.Port == 0 {
					ck.Port = 21
				}
				for _, f := range ck.File {
					if f.Regex != "" && f.Hash != "" {
						return errors.New("can't have both regex and hash for ftp file check")
					}
				}
				conf.Box[i].CheckList[j] = ck
			case checks.Imap:
				ck := c.(checks.Imap)
				ck.IP = b.IP
				if ck.Display == "" {
					ck.Display = "imap"
				}
				if ck.Name == "" {
					ck.Name = b.Name + "-" + ck.Display
				}
				if ck.Port == 0 {
					ck.Port = 143
				}
				conf.Box[i].CheckList[j] = ck
			case checks.Ldap:
				ck := c.(checks.Ldap)
				ck.IP = b.IP
				if ck.Display == "" {
					ck.Display = "ldap"
				}
				if ck.Name == "" {
					ck.Name = b.Name + "-" + ck.Display
				}
				if ck.Port == 0 {
					ck.Port = 636
				}
				if ck.Anonymous {
					return errors.New("anonymous ldap not supported")
				}
				conf.Box[i].CheckList[j] = ck
			case checks.Ping:
				ck := c.(checks.Ping)
				ck.IP = b.IP
				ck.Anonymous = true
				if ck.Count == 0 {
					ck.Count = 1
				}
				if ck.Display == "" {
					ck.Display = "ping"
				}
				if ck.Name == "" {
					ck.Name = b.Name + "-" + ck.Display
				}
				conf.Box[i].CheckList[j] = ck
			case checks.Rdp:
				ck := c.(checks.Rdp)
				ck.IP = b.IP
				if ck.Display == "" {
					ck.Display = "rdp"
				}
				if ck.Name == "" {
					ck.Name = b.Name + "-" + ck.Display
				}
				if ck.Port == 0 {
					ck.Port = 3389
				}
				conf.Box[i].CheckList[j] = ck
			case checks.Smb:
				ck := c.(checks.Smb)
				ck.IP = b.IP
				if ck.Display == "" {
					ck.Display = "smb"
				}
				if ck.Name == "" {
					ck.Name = b.Name + "-" + ck.Display
				}
				if ck.Port == 0 {
					ck.Port = 445
				}
				conf.Box[i].CheckList[j] = ck
			case checks.Smtp:
				ck := c.(checks.Smtp)
				ck.IP = b.IP
				if ck.Display == "" {
					ck.Display = "smtp"
				}
				if ck.Name == "" {
					ck.Name = b.Name + "-" + ck.Display
				}
				if ck.Port == 0 {
					ck.Port = 25
				}
				conf.Box[i].CheckList[j] = ck
			case checks.Sql:
				ck := c.(checks.Sql)
				ck.IP = b.IP
				if ck.Display == "" {
					ck.Display = "sql"
				}
				if ck.Name == "" {
					ck.Name = b.Name + "-" + ck.Display
				}
				if ck.Kind == "" {
					ck.Kind = "mysql"
				}
				if ck.Port == 0 {
					ck.Port = 3306
				}
				for _, q := range ck.Query {
					if q.UseRegex {
						regexp.MustCompile(q.Output)
					}
					if q.UseRegex && q.Contains {
						return errors.New("cannot use both regex and contains")
					}
				}
				conf.Box[i].CheckList[j] = ck
			case checks.Ssh:
				ck := c.(checks.Ssh)
				ck.IP = b.IP
				if ck.Display == "" {
					ck.Display = "ssh"
				}
				if ck.Name == "" {
					ck.Name = b.Name + "-" + ck.Display
				}
				if ck.Port == 0 {
					ck.Port = 22
				}
				if ck.PrivKey != "" && ck.BadAttempts != 0 {
					return errors.New("can not have bad attempts with pubkey for ssh")
				}
				for _, r := range ck.Command {
					if r.UseRegex {
						regexp.MustCompile(r.Output)
					}
					if r.UseRegex && r.Contains {
						return errors.New("cannot use both regex and contains")
					}
				}
				if ck.Anonymous {
					return errors.New("anonymous ssh not supported")
				}
				conf.Box[i].CheckList[j] = ck
			case checks.Tcp:
				ck := c.(checks.Tcp)
				ck.IP = b.IP
				ck.Anonymous = true
				if ck.Display == "" {
					ck.Display = "tcp"
				}
				if ck.Name == "" {
					ck.Name = b.Name + "-" + ck.Display
				}
				if ck.Port == 0 {
					return errors.New("tcp port required")
				}
				conf.Box[i].CheckList[j] = ck
			case checks.Vnc:
				ck := c.(checks.Vnc)
				ck.IP = b.IP
				if ck.Display == "" {
					ck.Display = "vnc"
				}
				if ck.Name == "" {
					ck.Name = b.Name + "-" + ck.Display
				}
				if ck.Port == 0 {
					ck.Port = 5900
				}
				conf.Box[i].CheckList[j] = ck
			case checks.Web:
				ck := c.(checks.Web)
				ck.IP = b.IP
				if ck.Display == "" {
					ck.Display = "web"
				}
				if ck.Name == "" {
					ck.Name = b.Name + "-" + ck.Display
				}
				if ck.Port == 0 {
					ck.Port = 80
				}
				if len(ck.Url) == 0 {
					return errors.New("no urls specified for web check " + ck.Name)
				}
				if ck.CredList == "" {
					ck.Anonymous = true
				}
				if ck.Scheme == "" {
					ck.Scheme = "http"
				}
				for _, u := range ck.Url {
					if u.Diff != 0 && u.CompareFile == "" {
						return errors.New("need compare file for diff in web")
					}
				}
				conf.Box[i].CheckList[j] = ck
			case checks.WinRM:
				ck := c.(checks.WinRM)
				ck.IP = b.IP
				if ck.Display == "" {
					ck.Display = "winrm"
				}
				if ck.Name == "" {
					ck.Name = b.Name + "-" + ck.Display
				}
				if ck.Port == 0 {
					if ck.Encrypted {
						ck.Port = 443
					} else {
						ck.Port = 80
					}
				}
				if ck.Anonymous {
					return errors.New("anonymous winrm not supported")
				}
				for _, r := range ck.Command {
					if r.UseRegex {
						regexp.MustCompile(r.Output)
					}
					if r.UseRegex && r.Contains {
						return errors.New("cannot use both regex and contains")
					}
				}
				conf.Box[i].CheckList[j] = ck
			}
		}
	}

	// look for duplicate checks
	for i, b := range conf.Box {
		for j := 0; j < len(b.CheckList)-1; j++ {
			if b.CheckList[j].FetchName() == b.CheckList[j+1].FetchName() {
				return errors.New("duplicate check name '" + b.CheckList[j].FetchName() + "' and '" + b.CheckList[j+1].FetchName() + "' for box " + b.Name)
			}
		}
		// sort checks
		conf.Box[i].CheckList = sortChecks(b.CheckList)
	}

	checks.CredLists = dwConf.Creds

	return nil
}

func getCheckName(check checks.Check) string {
	name := strings.Split(reflect.TypeOf(check).String(), ".")[1]
	fmt.Println("name is ", name)
	return name
}

func (m *config) GetFullIP(boxIP, teamIP string) string {
	return strings.Replace(boxIP, "x", teamIP, 1)
}
