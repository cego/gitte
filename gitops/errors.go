package gitops

import "strings"

// IsTransientError reports whether err looks like a transient network or SSH
// error that is worth retrying automatically.
func IsTransientError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	for _, pat := range transientPatterns {
		if strings.Contains(msg, pat) {
			return true
		}
	}
	return false
}

var transientPatterns = []string{
	"connection timed out",
	"connection refused",
	"connection reset by peer",
	"no route to host",
	"network is unreachable",
	"ssh: handshake failed",
	"ssh: unable to authenticate",
	"could not read from remote repository",
	"the remote end hung up unexpectedly",
	"temporary failure in name resolution",
	"failed to connect",
	"i/o timeout",
	"broken pipe",
	"packet_write_wait",
	"kex_exchange_identification",
}
