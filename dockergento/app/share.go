package app

import (
	"fmt"
	"regexp"
	"strconv"
	"time"

	"github.com/hiberus-magento/hiberus-dockergento/dockergento/core"
	"github.com/hiberus-magento/hiberus-dockergento/dockergento/ports"
)

//
// A public address for a project, for as long as somebody holds it open.
//
// Two things people actually need: showing progress to a client without deploying, and receiving
// real webhooks from payment gateways and marketplaces, which cannot reach a local environment.
//
// Cloudflare quick tunnels: no account, no credentials, nothing installed on the machine. The URL
// changes every time, which is right for a demo or for capturing webhooks and wrong for anything
// permanent.
//

const (
	shareImage = "cloudflare/cloudflared:latest"

	// shareWait is how long the edge is given to accept the tunnel, read every shareEvery
	shareWait  = 60 * time.Second
	shareEvery = 2 * time.Second
)

// shareAddress is how the tunnel reports where it answers, once the edge has accepted it.
var shareAddress = regexp.MustCompile(`https://[a-z0-9-]+\.trycloudflare\.com`)

// Sharer puts a project on the public internet.
type Sharer struct {
	Engine ports.ContainerEngine
	Relays ports.Relays

	// Say is what it reports on the way, and Binary the name the tool was invoked as.
	Say    func(string)
	Binary string

	// Sleep is the wait between two reads of the tunnel's output. Nil is time.Sleep.
	Sleep func(time.Duration)
}

// ShareRelay is the name of the container holding a project's tunnel.
func ShareRelay(project core.Project) string {
	return "hm-share-" + project.Name
}

// Open puts the project's web service on a public address.
//
// confirm is asked, with the project's name, before anything is exposed, and only once the service
// is known to be running; nil means there is nobody to ask. While this is open, anyone with the
// address reaches the environment, admin panel and data included, so a no stops everything.
//
// What it opens stays open when it returns: the caller holds it with Wait and ends it with Close.
func (s Sharer) Open(project core.Project, service string, port int, confirm func(string) bool) (core.Share, error) {
	web, err := runningContainer(s.Engine, project, service, s.Binary)
	if err != nil {
		return core.Share{}, err
	}

	if confirm != nil && !confirm(project.Name) {
		s.say("Nothing was shared.\n")

		return core.Share{Project: project.Name, Declined: true}, nil
	}

	relay := ShareRelay(project)

	// Anything a closed window left behind
	s.Relays.Remove(relay, false) //nolint:errcheck

	if len(web.Networks) == 0 {
		return core.Share{}, core.Refusal{
			Kind:    "no_network",
			Code:    5,
			Message: fmt.Sprintf("The '%s' service is not on any network", service),
			Hint:    s.Binary + " start",
		}
	}

	s.say("Opening a tunnel...\n")

	// The Host header is rewritten to the project's domain, or nginx has no vhost to match and
	// answers with whatever its default is
	host := project.Domain
	if host == "" {
		host = "localhost"
	}

	if err := s.Relays.Start(core.Relay{
		Name:    relay,
		Image:   shareImage,
		Network: web.Networks[0],
		Command: []string{"tunnel", "--no-autoupdate",
			"--url", "http://" + service + ":" + strconv.Itoa(port),
			"--http-host-header", host},
	}); err != nil {
		return core.Share{}, core.Refusal{
			Kind:    "tunnel_failed",
			Code:    3,
			Message: "The tunnel could not be started",
			Hint:    "docker pull " + shareImage,
		}
	}

	address := s.address(relay)
	if address == "" {
		s.Relays.Remove(relay, false) //nolint:errcheck

		return core.Share{}, core.Refusal{
			Kind:    "no_address",
			Code:    1,
			Message: "The tunnel did not report a public address",
			Hint:    "docker logs " + relay,
		}
	}

	return core.Share{URL: address, Project: project.Name, Relay: relay}, nil
}

// Wait returns when the tunnel stops, which is when somebody closed it.
func (s Sharer) Wait(relay string) error {
	return s.Relays.Wait(relay)
}

// Close takes the project's tunnel away and reports whether there was one.
func (s Sharer) Close(project core.Project) (bool, error) {
	return s.Relays.Remove(ShareRelay(project), false)
}

// address reads the tunnel's output until it says where it answers, or until it has been given
// long enough.
func (s Sharer) address(relay string) string {
	for waited := time.Duration(0); waited < shareWait; waited += shareEvery {
		logs, err := s.Relays.Logs(relay)
		if err != nil {
			// Gone: closed from somewhere else, an interruption among others
			return ""
		}

		if found := shareAddress.FindString(logs); found != "" {
			return found
		}

		s.sleep(shareEvery)
	}

	return ""
}

func (s Sharer) sleep(duration time.Duration) {
	if s.Sleep != nil {
		s.Sleep(duration)

		return
	}

	time.Sleep(duration)
}

func (s Sharer) say(message string) {
	if s.Say != nil {
		s.Say(message)
	}
}
