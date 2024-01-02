package sentinel

import "net/http"

func (stClient *TunnellingClient) Health(w http.ResponseWriter, _ *http.Request) {
	_, _ = w.Write([]byte("OK\n"))
}
