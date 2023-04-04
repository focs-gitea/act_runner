package ver

var (
	// go build -ldflags "-X gitea.com/gitea/act_runner/internal/pkg/ver.version=1.2.3"
	version = "dev"
)

func Version() string {
	return version
}
