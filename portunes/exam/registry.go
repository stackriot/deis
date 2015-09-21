package exam

import (
	"github.com/Masterminds/cookoo"
)

// Register registers all known exams in this package.
func Register(reg *cookoo.Registry) {
	reg.AddRoute(cookoo.Route{
		Name: "Pod Self Test",
		Help: "Test that this is a pod and can discover itself.",
		Does: []cookoo.Task{
			cookoo.Cmd{Name: "aboutme", Fn: AboutMeExam},
		},
	})
}
