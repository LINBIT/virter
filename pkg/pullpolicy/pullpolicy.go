package pullpolicy

import (
	"fmt"
	"strings"

	"github.com/LINBIT/containerapi"
)

const (
	Always     PullPolicy = "Always"
	IfNotExist PullPolicy = "IfNotExist"
	Never      PullPolicy = "Never"
)

type PullPolicy string

func (p *PullPolicy) UnmarshalText(text []byte) error {
	return p.Set(string(text))
}

func (p *PullPolicy) String() string {
	return string(*p)
}

func (p *PullPolicy) Set(s string) error {
	switch strings.ToLower(s) {
	case strings.ToLower(string(Never)), strings.ToLower(string(IfNotExist)), strings.ToLower(string(Always)):
		*p = PullPolicy(s)
		return nil
	default:
		return fmt.Errorf("unknown pull policy. [%s, %s, %s]", Always, IfNotExist, Never)
	}
}

func (p *PullPolicy) Type() string {
	return "pullPolicy"
}

func (p PullPolicy) ForContainer() containerapi.ShouldPull {
	switch p {
	case Always:
		return containerapi.PullAlways
	case IfNotExist:
		return containerapi.PullIfNotExists
	case Never:
		return containerapi.PullNever
	default:
		return nil
	}
}
