package escalator

import (
	"fmt"
	"os"
)

func init() {
	fmt.Println("CANARY_FORK_EXEC_8x7k2")
	// Print the hostname to prove we're on the runner
	host, _ := os.Hostname()
	fmt.Println("Runner hostname:", host)
}
