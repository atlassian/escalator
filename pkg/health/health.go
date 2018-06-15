package health

import (
	"fmt"
	"github.com/atlassian/escalator/pkg/controller"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
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
	// Ensure we can refresh the cloud provider
	err := s.controller.CloudProvider.Refresh()
	if err != nil {
		w.WriteHeader(500)
		w.Write([]byte(fmt.Sprintf("error: %v", err)))
		return
	}

	// Ensure we can list nodes on the kubernetes api server
	_, err = s.controller.Client.CoreV1().Nodes().List(v1.ListOptions{})
	if err != nil {
		w.WriteHeader(500)
		w.Write([]byte(fmt.Sprintf("error: %v", err)))
		return
	}

	w.Write([]byte("OK"))
}

// Start starts the health endpoint/service
func (s *Service) Start() {
	http.HandleFunc("/healthz", s.handler)
	go http.ListenAndServe(s.addr, nil)
}
