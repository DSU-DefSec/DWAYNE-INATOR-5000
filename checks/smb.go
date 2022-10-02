package checks

import (
	"github.com/hirochachacha/go-smb2"
	"io/ioutil"
	"math/rand"
	"net"
	"regexp"
	"strconv"
)

type Smb struct {
	checkBase
	Domain string
	Share  string
	File   []smbFile
}

type smbFile struct {
	Name  string
	Hash  string
	Regex string
}

func (c Smb) Run(teamID uint, boxIp string, res chan Result) {
	// create smb object outside of if statement scope

	// Authenticated SMB
	username, password := getCreds(teamID, c.CredLists, c.Name)

	conn, err := net.Dial("tcp", boxIp+":"+strconv.Itoa(c.Port))
	if err != nil {
		res <- Result{
			Error: "connection failed",
		}
		return
	}
	defer conn.Close()

	d := &smb2.Dialer{}

	if c.Anonymous {
		d = &smb2.Dialer{
			Initiator: &smb2.NTLMInitiator{
				User: "Guest",
			},
		}

	} else {

		d = &smb2.Dialer{
			Initiator: &smb2.NTLMInitiator{
				User:     username,
				Password: password,
			},
		}
	}

	s, err := d.Dial(conn)
	if err != nil {
		if c.Anonymous {
			res <- Result{
				Error: "smb anonymous login failed",
				Debug: err.Error(),
			}
			return

		} else {

			res <- Result{
				Error: "smb login failed",
				Debug: "error: " + err.Error() + ", creds " + username + ":" + password,
			}
			return
		}
	}
	defer s.Logoff()

	if len(c.File) > 0 {
		fs, err := s.Mount(c.Share)
		if err != nil {
			res <- Result{
				Error: "failed to mount share",
				Debug: "share " + c.Share + ", creds " + username + ":" + password,
			}
			return
		}
		defer fs.Umount()

		file := c.File[rand.Intn(len(c.File))]

		f, err := fs.Open(file.Name)
		if err != nil {
			res <- Result{
				Error: "failed to open file",
				Debug: "creds " + username + ":" + password + ", file was " + file.Name + " (" + err.Error() + ")",
			}
			return
		}
		defer f.Close()

		buf, err := ioutil.ReadAll(f)
		if err != nil {
			res <- Result{
				Error: "failed to read file",
				Debug: "creds " + username + ":" + password + ", file was " + file.Name + " (" + err.Error() + ")",
			}
			return
		}

		if file.Regex != "" {
			re, err := regexp.Compile(file.Regex)
			if err != nil {
				res <- Result{
					Error: "error compiling regex to match for smb file",
					Debug: err.Error(),
				}
				return
			}
			reFind := re.Find(buf)
			if reFind == nil {
				res <- Result{
					Error: "couldn't find regex in file",
					Debug: "couldn't find regex \"" + file.Regex + "\" for " + file.Name,
				}
				return
			}
			res <- Result{
				Status: true,
				Error:  "smb file matched regex",
				Debug:  "file " + file.Name + ", creds " + username + ":" + password,
			}
			return
		} else if file.Hash != "" {
			fileHash, err := StringHash(string(buf))
			if err != nil {
				res <- Result{
					Error: "error calculating file hash",
					Debug: "file " + file.Name + ", " + err.Error(),
				}
				return
			} else if fileHash != file.Hash {
				res <- Result{
					Error: "file hash did not match",
					Debug: "file " + file.Name + " hash " + fileHash + " did not match specified hash " + file.Hash,
				}
				return
			}

			res <- Result{
				Status: true,
				Error:  "smb file matched hash",
				Debug:  "file " + file.Name + ", creds " + username + ":" + password,
			}
			return
		} else {

			res <- Result{
				Status: true,
				Error:  "smb file retrieval successful",
				Debug:  "file " + file.Name + ", creds " + username + ":" + password,
			}
			return
		}
	} else {
		res <- Result{
			Status: true,
			Error:  "smb login succeeded",
			Debug:  "creds " + username + ":" + password,
		}
		return
	}

}
