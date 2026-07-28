package ratio_setting

import (
	"strings"

	"github.com/QuantumNous/new-api/common"
)

const CompactModelSuffix = common.OpenAIResponseCompactModelSuffix
const CompactWildcardModelKey = "*" + CompactModelSuffix

func WithCompactModelSuffix(modelName string) string {
	if strings.HasSuffix(modelName, CompactModelSuffix) {
		return modelName
	}
	return modelName + CompactModelSuffix
}
