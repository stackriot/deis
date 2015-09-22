package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/deis/deis/builder/env"
	"github.com/deis/deis/pkg/aboutme"
	"github.com/deis/deis/pkg/etcd/discovery"

	"github.com/Masterminds/cookoo"
	"github.com/Masterminds/cookoo/log"
)

func main() {
	reg, router, c := cookoo.Cookoo()
	routes(reg)

	router.HandleRequest("boot", c, false)
}

func routes(reg *cookoo.Registry) {
	reg.AddRoute(cookoo.Route{
		Name: "boot",
		Help: "Boot Etcd",
		Does: []cookoo.Task{
			cookoo.Cmd{
				Name: "setenv",
				Fn:   iam,
			},
			// This synchronizes the local copy of env vars with the actual
			// environment. So all of these vars are available in cxt: or in
			// os.Getenv(). They will also be available to the etcd process
			// we spawn.
			cookoo.Cmd{
				Name: "vars",
				Fn:   env.Get,
				Using: []cookoo.Param{
					{Name: "DEIS_ETCD_DISCOVERY_SERVICE_HOST"},
					{Name: "DEIS_ETCD_DISCOVERY_SERVICE_PORT"},
					{Name: "DEIS_ETCD_1_SERVICE_HOST"},
					{Name: "DEIS_ETCD_2_SERVICE_HOST"},
					{Name: "DEIS_ETCD_3_SERVICE_HOST"},
					{Name: "DEIS_ETCD_1_SERVICE_PORT_CLIENT"},
					{Name: "DEIS_ETCD_2_SERVICE_PORT_CLIENT"},
					{Name: "DEIS_ETCD_3_SERVICE_PORT_CLIENT"},

					// This should be set in Kubernetes environment.
					{Name: "ETCD_NAME", DefaultValue: "deis1"},

					// Peer URLs are for traffic between etcd nodes.
					// These point to internal IP addresses, not service addresses.
					{Name: "ETCD_LISTEN_PEER_URLS", DefaultValue: "http://$MY_IP:$MY_PORT_PEER"},
					{Name: "ETCD_INITIAL_ADVERTISE_PEER_URLS", DefaultValue: "http://$MY_IP:$MY_PORT_PEER"},

					// This is for static cluster. Delete if we go with discovery.
					/*
						{
							Name:         "ETCD_INITIAL_CLUSTER",
							DefaultValue: "deis1=http://$DEIS_ETCD_1_SERVICE_HOST:$DEIS_ETCD_1_SERVICE_PORT_PEER,deis2=http://$DEIS_ETCD_2_SERVICE_HOST:$DEIS_ETCD_2_SERVICE_PORT_PEER,deis3=http://$DEIS_ETCD_3_SERVICE_HOST:$DEIS_ETCD_3_SERVICE_PORT_PEER",
						},
						{Name: "ETCD_INITIAL_CLUSTER_STATE", DefaultValue: "new"},
						{Name: "ETCD_INTIIAL_CLUSTER_TOKEN", DefaultValue: "c0ff33"},
					*/

					// These point to service addresses.
					{
						Name:         "ETCD_LISTEN_CLIENT_URLS",
						DefaultValue: "http://$MY_IP:$MY_PORT_CLIENT,http://127.0.0.1:$MY_PORT_CLIENT",
					},
					{Name: "ETCD_ADVERTISE_CLIENT_URLS", DefaultValue: "http://$MY_IP:$MY_PORT_CLIENT"},

					// {Name: "ETCD_WAL_DIR", DefaultValue: "/var/"},
					// {Name: "ETCD_MAX_WALS", DefaultValue: "5"},
				},
			},
			cookoo.Cmd{
				Name: "discoveryURL",
				Fn:   discoveryURL,
				Using: []cookoo.Param{
					{Name: "host", From: "cxt:DEIS_ETCD_DISCOVERY_SERVICE_HOST"},
					{Name: "port", From: "cxt:DEIS_ETCD_DISCOVERY_SERVICE_PORT"},
				},
			},
			cookoo.Cmd{
				Name: "vars2",
				Fn:   env.Get,
				Using: []cookoo.Param{
					{Name: "ETCD_DISCOVERY", From: "cxt:discoveryURL"},
				},
			},
			cookoo.Cmd{
				Name: "startEtcd",
				Fn:   startEtcd,
				Using: []cookoo.Param{
					{Name: "discover", From: "cxt:discoveryUrl"},
				},
			},
		},
	})
}

// iam injects info into the environment about a host's self.
//
// Sets the following environment variables:
//
//	MY_IP
//	MY_SERVICE_IP
// 	MY_PORT_PEER
// 	MY_PORT_CLIENT
func iam(c cookoo.Context, p *cookoo.Params) (interface{}, cookoo.Interrupt) {
	name := os.Getenv("ETCD_NAME")
	ip, err := aboutme.MyIP()
	if err != nil {
		return nil, err
	}
	os.Setenv("MY_IP", ip)
	os.Setenv("ETCD_NAME", os.Getenv("HOSTNAME"))

	// TODO: Swap this with a regexp once the final naming convention has
	// been decided.
	var index int
	switch name {
	case "deis1":
		index = 1
	case "deis2":
		index = 2
	case "deis3":
		index = 3
	default:
		log.Info(c, "Can't get $ETCD_NAME. Initializing defaults.")
		os.Setenv("MY_IP", ip)
		os.Setenv("MY_SERVICE_IP", "127.0.0.1")
		os.Setenv("MY_PORT_PEER", "2380")
		os.Setenv("MY_PORT_CLIENT", "4100")
		return nil, nil
	}

	passEnv("MY_SERVICE_IP", fmt.Sprintf("$DEIS_ETCD_%d_SERVICE_HOST", index))
	passEnv("MY_PORT_CLIENT", fmt.Sprintf("$DEIS_ETCD_%d_SERVICE_PORT_CLIENT", index))
	passEnv("MY_PORT_PEER", fmt.Sprintf("$DEIS_ETCD_%d_SERVICE_PORT_PEER", index))
	return nil, nil
}

func passEnv(newName, passthru string) {
	os.Setenv(newName, os.ExpandEnv(passthru))
}

// startEtcd starts a cluster member of a static etcd cluster.
//
// Params:
// 	- discover (string): Value to pass to etcd --discovery.
func startEtcd(c cookoo.Context, p *cookoo.Params) (interface{}, cookoo.Interrupt) {
	// Use config from environment.
	cmd := exec.Command("etcd")
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout

	println(strings.Join(os.Environ(), "\n"))
	if err := cmd.Start(); err != nil {
		log.Errf(c, "Failed to start etcd: %s", err)
		return nil, err
	}

	if err := cmd.Wait(); err != nil {
		log.Errf(c, "Etcd quit unexpectedly: %s", err)
	}
	return nil, nil
}

// disoveryURL gets the URL to the y service.
func discoveryURL(c cookoo.Context, p *cookoo.Params) (interface{}, cookoo.Interrupt) {
	token, err := discovery.Token()
	if err != nil {
		return nil, err
	}
	host := p.Get("host", "").(string)
	port := p.Get("port", "2379").(string)
	u := fmt.Sprintf(discovery.ClusterDiscoveryURL, host, port, token)
	log.Infof(c, "Discovery URL: %s", u)

	return u, nil
}
