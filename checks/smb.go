package checks

type Smb struct {
	checkBase
	Port   int
	Domain string
	// ??
}

func (c Smb) Run(teamPrefix string, res chan Result) {
	// Authenticated SMB
	if len(c.CredLists) > 0 {
		username, password := getCreds(c.CredLists)
		// log in smb
		// if err != nil {
			// return bad result
		// check if file is specified
			// retrieve file
		// check if hash is specified
			// compare hash
		res <- Result{
			Status: true,
			Debug:  "creds used were " + username + ":" + password,
		}
		return
	}
	res <- Result{
		Status: true,
		Debug:  "anonymous smb ran",
	}
	// anonymous smb
}
