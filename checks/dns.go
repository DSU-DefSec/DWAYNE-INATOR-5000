package checks

type Dns struct {
	checkBase
	Port   int
	Domain string
	// ??
}

func (c Dns) Run(teamPrefix string, res chan Result) {
	res <- Result{
		Status: true,
		Debug:  "check ran",
	}
}
