# Design

## The proxy is an ordinary compose project

It has a name anybody recognises in `docker ps`, a compose file on disk, and it can be stopped by
hand without magic. So it is brought up and down through the same orchestrator every project uses,
with a synthetic project pointing at its own directory — rather than through a second path that
would drift from the first.

## Asking the router rather than the containers

`status` reads Traefik's API on the loopback address the proxy publishes it on. The alternative —
listing containers with routing labels — answers a different question: it says what asked to be
routed, not what is. A router that failed to come up carries exactly the same labels.

## What is not here

Writing a project's overlay, and signing certificates. The first belongs where a project is set
up, and the second needs a tool that can sign one. What is here is the half that tells the proxy
where a certificate lives, because that is the proxy's own configuration.
