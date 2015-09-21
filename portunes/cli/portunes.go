package main

import (
	"fmt"
	"io/ioutil"
	"os"

	"github.com/Masterminds/cookoo"
	"github.com/codegangsta/cli"
	"github.com/deis/deis/portunes"
	"github.com/deis/deis/portunes/exam"

	"gopkg.in/yaml.v2"
)

func main() {
	app := cli.NewApp()
	app.Name = "portunes"
	app.Usage = "The Portunes Deis testing tool"
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:  "config",
			Value: "portunes.yaml",
			Usage: "Path to Portunes configuration file.",
		},
	}
	app.Action = func(c *cli.Context) {
		// Load from a YAML file.
		b, err := batteryFromYAML(c.String("config"))
		if err != nil {
			fmt.Printf("Failed to parse configuration file: %s\n", err)
			os.Exit(1)
		}

		p := portunes.New(b)
		register(p.Registry)

		cancel := make(chan bool)
		addr := ":8090"

		fmt.Println("Starting HTTP server on ", addr)
		fmt.Println("Starting test harness")
		if err := p.Run(cancel); err != nil {
			fmt.Printf("Error: %s\n", err)
		}
		p.ServeHTTP(addr)
	}
	app.Run(os.Args)
}

func register(reg *cookoo.Registry) {
	reg.AddRoute(cookoo.Route{
		Name: "HTTP Self Test",
		Does: []cookoo.Task{
			cookoo.Cmd{Name: "http", Fn: PingLocal},
		},
	})
	exam.Register(reg)
}

func batteryFromYAML(filename string) (*portunes.Battery, error) {
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	var battery portunes.Battery
	return &battery, yaml.Unmarshal(data, &battery)
}

// PingLocal checks that the local HTTP server is running.
func PingLocal(c cookoo.Context, p *cookoo.Params) (interface{}, cookoo.Interrupt) {
	return true, nil
}
