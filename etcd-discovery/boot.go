package main

import (
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"time"

	"github.com/coreos/go-etcd/etcd"
	"github.com/deis/deis/pkg/etcd/discovery"
)

func main() {

	ip, err := myIP()
	if err != nil {
		log.Printf("Failed to start because could not get IP: %s", err)
		os.Exit(321)
	}

	port := os.Getenv("DEIS_ETCD_CLIENT_PORT")
	if port == "" {
		port = "2381"
	}

	aurl := fmt.Sprintf("http://%s:%s", ip, port)
	curl := fmt.Sprintf("http://%s:%s,http://localhost:%s", ip, port, port)

	cmd := exec.Command("etcd", "-advertise-client-urls", aurl, "-listen-client-urls", curl)
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	go func() {
		err := cmd.Start()
		if err != nil {
			log.Printf("Failed to start etcd: %s", err)
			os.Exit(2)
		}
	}()

	// Give etcd time to start up.
	log.Print("Sleeping for 5 seconds...")
	time.Sleep(5 * time.Second)
	log.Print("I'm awake.")

	uuid, err := discovery.Token()
	if err != nil {
		log.Printf("Failed to read %s", discovery.TokenFile)
		os.Exit(404)
	}
	size := os.Getenv("DEIS_ETCD_CLUSTER_SIZE")
	if size == "" {
		size = "3"
	}

	key := fmt.Sprintf(discovery.ClusterSizeKey, uuid)
	cli := etcd.NewClient([]string{"http://localhost:2381"})
	if _, err := cli.Create(key, size, 0); err != nil {
		log.Printf("Failed to add key: %s", err)
	}

	log.Printf("The etcd-discovery service is now ready and waiting.")
	if err := cmd.Wait(); err != nil {
		log.Printf("Etcd stopped running: %s", err)
	}
}

// myIP returns the IP assigned to eth0.
//
// This is OS specific (Linux in a container gets eth0).
func myIP() (string, error) {
	iface, err := net.InterfaceByName("eth0")
	if err != nil {
		return "", err
	}
	addrs, err := iface.Addrs()
	if err != nil {
		return "", err
	}
	var ip string
	for _, a := range addrs {
		if ipnet, ok := a.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				ip = ipnet.IP.String()
			}
		}
	}
	if len(ip) == 0 {
		return ip, errors.New("Found no IPv4 addresses.")
	}
	return ip, nil
}
