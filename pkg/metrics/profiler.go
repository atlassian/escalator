package metrics

import (
	"net/http"
)


type Profiler interface {
	InstallHTTP(mux *http.ServeMux)
}
