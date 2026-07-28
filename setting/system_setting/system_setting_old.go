package system_setting

import (
	"strings"

	"github.com/QuantumNous/new-api/common"
)

var ServerAddress = "http://localhost:3000"
var APIAddress = common.GetEnvOrDefaultString("API_ADDRESS", "")
var WorkerUrl = ""
var WorkerValidKey = ""
var WorkerAllowHttpImageRequestEnabled = false

func GetAPIAddress() string {
	apiAddress := strings.TrimRight(strings.TrimSpace(APIAddress), "/")
	if apiAddress != "" {
		return apiAddress
	}
	return strings.TrimRight(strings.TrimSpace(ServerAddress), "/")
}

func EnableWorker() bool {
	return WorkerUrl != ""
}
