package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/Masterminds/cookoo"
	"github.com/Masterminds/cookoo/log"
	"github.com/coreos/etcd/client"
	"github.com/deis/deis/pkg/aboutme"
	"github.com/deis/deis/pkg/env"
	"github.com/deis/deis/pkg/etcd"
	"github.com/deis/deis/pkg/etcd/discovery"
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
			cookoo.Cmd{
				Name: "discoveryToken",
				Fn:   discovery.GetToken,
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
					{Name: "HOSTNAME"},

					{Name: "DEIS_ETCD_CLUSTER_SIZE", DefaultValue: "3"},
					{Name: "DEIS_ETCD_DISCOVERY_TOKEN", From: "cxt:discoveryToken"},

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

			// We need to connect to the discovery service to find out whether
			// we're part of a new cluster, or part of an existing cluster.
			cookoo.Cmd{
				Name: "discoveryClient",
				Fn:   etcd.CreateClient,
				Using: []cookoo.Param{
					{
						Name:         "url",
						DefaultValue: "http://$DEIS_ETCD_DISCOVERY_SERVICE_HOST:$DEIS_ETCD_DISCOVERY_SERVICE_PORT",
					},
				},
			},
			cookoo.Cmd{
				Name: "clusterClient",
				Fn:   etcd.CreateClient,
				Using: []cookoo.Param{
					{
						Name:         "url",
						DefaultValue: "http://$DEIS_ETCD_1_SERVICE_HOST:$DEIS_ETCD_1_SERVICE_PORT",
					},
				},
			},
			cookoo.Cmd{
				Name: "vars2",
				Fn:   env.Get,
				Using: []cookoo.Param{
					{
						Name:         "ETCD_DISCOVERY",
						DefaultValue: "http://$DEIS_ETCD_DISCOVERY_SERVICE_HOST:$DEIS_ETCD_DISCOVERY_SERVICE_PORT/v2/keys/deis/discovery/$DEIS_ETCD_DISCOVERY_TOKEN",
					},
				},
			},
			cookoo.Cmd{
				Name: "joinMode",
				Fn:   setJoinMode,
				Using: []cookoo.Param{
					{Name: "client", From: "cxt:discoveryClient"},
					{Name: "path", DefaultValue: "/deis/discovery/$DEIS_ETCD_DISCOVERY_TOKEN"},
					{Name: "desiredLen", From: "cxt:DEIS_ETCD_CLUSTER_SIZE"},
				},
			},
			// If joinMode is "new", we reroute to the route that creates new
			// clusters. Otherwise, we keep going on this chain, assuming
			// we're working with an existing cluster.
			cookoo.Cmd{
				Name: "rerouteIfNew",
				Fn: func(c cookoo.Context, p *cookoo.Params) (interface{}, cookoo.Interrupt) {
					if m := p.Get("joinMode", "").(string); m == "new" {
						return nil, cookoo.NewReroute("@newCluster")
					}
					return nil, nil
				},
				Using: []cookoo.Param{
					{Name: "joinMode", From: "cxt:joinMode"},
				},
			},
			cookoo.Cmd{
				Name: "removeMember",
				Fn:   etcd.RemoveMemberByName,
				Using: []cookoo.Param{
					{Name: "client", From: "cxt:clusterClient"},
					{Name: "name", From: "cxt:HOSTNAME"},
				},
			},
			cookoo.Cmd{
				Name: "initialCluster",
				Fn:   etcd.GetInitialCluster,
				Using: []cookoo.Param{
					{Name: "client", From: "cxt:clusterClient"},
				},
			},
			// TODO: Get ETCD_INITIAL_CLUSTER
			/*
				What we need to do here:
				- First, we need to check the discovery service to find out if
				  it is full.
				  - If the discovery service is not full, we set etcd into discovery
				    mode and let it join via the service.
				  - If it IS FULL, then we need to do several things:
				    - We need to get the service URL for etcd
					- We need to contact that URL and find out if our host name
					  is a member there.
					- If this name is already a member, remove the member.
					- Put self into joining mode, and give it as much of the
					  cluster as it can find
				- Start etcd
			*/

			cookoo.Cmd{
				Name: "startEtcd",
				Fn:   startEtcd,
				Using: []cookoo.Param{
					{Name: "discover", From: "cxt:discoveryUrl"},
				},
			},
		},
	})

	reg.AddRoute(cookoo.Route{
		Name: "@newCluster",
		Help: "Start as part of a new cluster",
		Does: []cookoo.Task{
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

// setJoinMode determines what mode to start the etcd server in.
//
// In discovery mode, this will use the discovery URL to join a new cluster.
// In "existing" mode, this will join to an existing cluster directly.
//
// Params:
//	- client (etcd.Getter): initialized etcd client
//	- path (string): path to get. This will go through os.ExpandEnv().
// 	- desiredLen (string): The number of nodes to expect in etcd. This is
// 	usually stored as a string.
//
// Returns:
//  string "existing" or "new"
func setJoinMode(c cookoo.Context, p *cookoo.Params) (interface{}, cookoo.Interrupt) {
	cli := p.Get("client", nil).(client.Client)
	dlen := p.Get("desiredLen", "3").(string)
	path := p.Get("path", "").(string)
	path = os.ExpandEnv(path)

	state := "existing"

	dint, err := strconv.Atoi(dlen)
	if err != nil {
		log.Warnf(c, "Expected integer length, got '%s'. Defaulting to 3", dlen)
		dint = 3
	}

	res, err := etcd.SimpleGet(cli, path, true)
	if err != nil {
		return state, err
	}

	if !res.Node.Dir {
		return state, errors.New("Expected a directory node in discovery service")
	}

	if len(res.Node.Nodes) < dint {
		state = "new"
	}

	os.Setenv("ETCD_INITIAL_CLUSTER_STATE", state)
	return state, nil
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
