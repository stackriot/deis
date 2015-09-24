/* Package discovery contains utlities for Etcd discovery. */
package discovery

import (
	"bytes"
	"io/ioutil"

	"github.com/Masterminds/cookoo"
)

const TokenFile = "/var/run/secrets/deis/etcd/discovery/token"
const ClusterDiscoveryURL = "http://%s:%s/v2/keys/deis/discovery/%s"
const ClusterSizeKey = "deis/discovery/%s/_config/size"

// Token reads the discovery token from the TokenFile and returns it.
func Token() ([]byte, error) {
	data, err := ioutil.ReadFile(TokenFile)
	if err != nil {
		return data, err
	}
	data = bytes.TrimSpace(data)
	return data, nil
}

// GetToken is a command to get a token.
//
// This is a convenience for calling Token in a route.
func GetToken(c cookoo.Context, p *cookoo.Params) (interface{}, cookoo.Interrupt) {
	return Token()
}
