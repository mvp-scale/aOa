package version

var (
	Version   = "dev"
	BuildDate = "unknown"
)

func String() string { return Version + " (" + BuildDate + ")" }
