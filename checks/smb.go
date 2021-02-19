package checks

import (
	"github.com/stacktitan/smb/smb"
)

type Smb struct {
	checkBase
	Domain string
	File   []smbFile
}

type smbFile struct {
	Name  string
	Hash  string
	Regex string
}

func (c Smb) Run(teamName, boxIp string, res chan Result) {
	// create smb object outside of if statement scope

	// Authenticated SMB
	if !c.Anonymous {
		username, password := getCreds(c.CredLists, teamName, c.Name)
		options := smb.Options{
			Host:        boxIp,
			Port:        445,
			User:        username,
			Domain:      c.Domain,
			Workstation: "",
			Password:    password,
		}
		session, err := smb.NewSession(options, false)
		if err != nil {
			res <- Result{
				Error: "smb session creation failed",
				Debug: "creds " + username + ":" + password,
			}
			return
		}
		defer session.Close()

		if session.IsAuthenticated {
			res <- Result{
				Status: true,
				Error:  "smb login succeeded",
				Debug:  "creds " + username + ":" + password,
			}
			return
		} else {
			res <- Result{
				Error: "smb login failed",
				Debug: "creds " + username + ":" + password + " for domain " + c.Domain,
			}
			return
		}
	}
	/*
			} else {
				// anonymous smb
				// PLACEHOLDER: test tcp only
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
		            Debug:  "responded to tcp request",
		        }
			}


		    if len(c.File) > 0 {
		        file := c.File[rand.Intn(len(c.File))]
		        r, err := conn.Retr(file.Name)
		        if err != nil {
		            res <- Result{
		                Status: true,
		                Error:  "ftp login suceeded",
		                Debug:  "creds used were " + username + ":" + password,
		            }
		            return
		        }
		        defer r.Close()
		        buf, err := ioutil.ReadAll(r)
		        if err != nil {
		            res <- Result{
		                Error: "failed to read ftp file",
		                Debug: "tried to read " + file.Name,
		            }
		            return
		        }
		        if file.Regex != "" {
		            re, err := regexp.Compile(file.Regex)
		            if err != nil {
		                res <- Result{
		                    Error: "error compiling regex to match for ftp file",
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
		        } else if file.Hash != ""{
		            fileHash, err := StringHash(string(buf))
		            if err != nil {
		                res <- Result{
		                    Error: "error calculating file hash",
		                    Debug: err.Error(),
		                }
		                return
		            } else if fileHash != file.Hash {
		                res <- Result{
		                    Error: "file hash did not match",
		                    Debug: "file hash " + fileHash + " did not match specified hash " + file.Hash,
		                }
		                return
		            }
		        }
		    }
		    res <- Result{
		        Status: true,
		        Debug:  "creds used were " + username + ":" + password,
		    }
	*/
}
