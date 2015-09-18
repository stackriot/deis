package aboutme

import (
	"errors"
	"net"
	"os"
)

type Me struct {
	ApiServer, Name                      string
	IP, NodeIP, Namespace, SelfLink, UID string
	Labels                               map[string]string
	Annotations                          map[string]string
}

// FromEnv uses the environment to create a new Me.
func FromEnv() *Me {
	host := os.Getenv("KUBERNETES_SERVICE_HOST")
	port := os.Getenv("KUBERNETES_SERVICE_PORT")
	name := os.Getenv("HOSTNAME")

	// FIXME: Better way? Probably scanning secrets for
	// an SSL cert would help?
	proto := "http"
	if port == "443" {
		proto = "https"
	}

	url := proto + "://" + host + ":" + port

	return &Me{
		ApiServer: url,
		Name:      name,
	}
}

// MyIP examines the local interfaces and guesses which is its IP.
//
// Containers tend to put the IP address in eth0, so this attempts to look up
// that interface and retrieve its IP. It is fairly naive. To get more
// thorough IP information, you may prefer to use the `net` package and
// look up the desired information.
//
// Because this queries the interfaces, not the Kube API server, this could,
// in theory, return an IP address different from Me.IP.
func MyIP() (string, error) {
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
