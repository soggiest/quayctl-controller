package spec

import (
	"errors"
	"fmt"
)

const (
	defaultBaseImage = ""
	defaultVersion   = ""

	TPRKind        = "pipeline"
	TPRKindPlural  = "pipelines"
	TPRGroup       = ".coreos.com"
	TPRVersion     = "v1beta1"
	TPRDescription = "Managed etcd clusters"
)

var (
	ErrBackupUnsetRestoreSet = errors.New("spec: backup policy must be set if restore p    olicy is set")
)

func TPRName() string {
	return fmt.Sprintf("%s.%s", TPRKind, TPRGroup)
}
