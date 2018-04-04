package health

import (
	"github.com/atlassian/escalator/pkg/controller"
	"net/http"
)

type Service struct {
	addr       string
	controller *controller.Controller
}

// NewHealthService creates a new health service
func NewHealthService(addr string, controller *controller.Controller) *Service {
	return &Service{
		addr:       addr,
		controller: controller,
	}
}

func (s *Service) handler(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("OK"))
}

// Start starts the health endpoint/service
func (s *Service) Start() {
	http.HandleFunc("/healthz", s.handler)
	go http.ListenAndServe(s.addr, nil)
}
