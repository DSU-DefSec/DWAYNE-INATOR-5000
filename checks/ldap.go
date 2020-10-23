package checks

type Ldap struct {
	checkBase
	Port   int
	Domain string
	// ??
}

func (c Ldap) Run(teamPrefix string, res chan Result) {
	// execute commands
	res <- Result{
		Status: true,
		Debug:  "check ran",
	}
}
