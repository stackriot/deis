package exam

import (
	"errors"
	"fmt"
	"os"

	"github.com/Masterminds/cookoo"
	"github.com/deis/deis/pkg/aboutme"
)

// AboutMeExam tests whether this pod can find out about itself.
func AboutMeExam(c cookoo.Context, p *cookoo.Params) (interface{}, cookoo.Interrupt) {
	me, err := aboutme.FromEnv()
	if err != nil {
		return false, fmt.Errorf("Failed FromEnv: %s", err)
	}

	if me.Name != os.Getenv("HOSTNAME") {
		return false, fmt.Errorf("Unexpected name: %s", me.Name)
	}

	ip, err := aboutme.MyIP()
	if err != nil {
		return false, err
	}
	if len(ip) == 0 {
		return false, errors.New("Unexpected empty IP address from eth0")
	}

	if len(me.Namespace) == 0 {
		return false, errors.New("Unexpected empty namespace")
	}

	if len(me.IP) == 0 {
		return false, errors.New("Unexpected empty IP")
	}

	return true, nil
}
