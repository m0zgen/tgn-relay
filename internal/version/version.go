package version

var (
	Version = "dev"
	Commit  = "none"
	Date    = "unknown"
)

func String() string {
	return "tgn-relay " + Version + " (" + Commit + ", " + Date + ")"
}
