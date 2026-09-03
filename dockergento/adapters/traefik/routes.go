// Package traefik asks the proxy what it is routing.
//
// From the proxy's own API and not from the container labels, because they are not the same
// question: a container with the labels and a router that never came up look identical from
// outside, and the one that matters is whether a request would arrive.
package traefik

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"sort"
	"time"

	"github.com/hiberus-magento/hiberus-dockergento/dockergento/core"
)

// API is the proxy's dashboard, bound to loopback.
type API struct {
	// Address is where it listens. Empty means where the proxy this tool starts puts it.
	Address string
}

// host pulls the name out of a rule. Traefik writes them as “Host(`shop.local`)“, sometimes with
// other matchers around it.
var host = regexp.MustCompile("Host\\(`([^`]*)`\\)")

type router struct {
	Rule     string `json:"rule"`
	Status   string `json:"status"`
	Provider string `json:"provider"`
}

// Routes is what the proxy is routing, one entry per domain, in a stable order.
func (a API) Routes() ([]core.Route, error) {
	address := a.Address
	if address == "" {
		address = "127.0.0.1:8080"
	}

	client := http.Client{Timeout: 3 * time.Second}

	response, err := client.Get(fmt.Sprintf("http://%s/api/http/routers", address))
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	var routers []router

	if err := json.NewDecoder(response.Body).Decode(&routers); err != nil {
		return nil, err
	}

	seen := map[string]bool{}
	routes := []core.Route{}

	for _, one := range routers {
		// Only what the containers declared. Traefik's own dashboard router is not something
		// anybody asked to be routed
		if one.Provider != "docker" {
			continue
		}

		found := host.FindStringSubmatch(one.Rule)
		if len(found) < 2 || found[1] == "" || seen[found[1]] {
			continue
		}

		seen[found[1]] = true
		routes = append(routes, core.Route{Host: found[1], Status: one.Status})
	}

	sort.Slice(routes, func(a, b int) bool { return routes[a].Host < routes[b].Host })

	return routes, nil
}
