package action

type DryRunMode string

var (
	DryRunModeNone   DryRunMode = "none"
	DryRunModeClient DryRunMode = "client"
	DryRunModeServer DryRunMode = "server"
)
