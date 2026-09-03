# Global proxy

## ADDED Requirements

### Requirement: The proxy is one thing, whichever half starts it

The tool SHALL generate the same proxy configuration from either implementation.

#### Scenario: Started by one and stopped by the other

- **WHEN** the proxy is started and then stopped through different halves of the tool
- **THEN** both act on the same compose project, and it is stopped

#### Scenario: The configuration has not changed

- **WHEN** the proxy is started again
- **THEN** its compose file is left alone rather than rewritten

### Requirement: What is in the way is named

The tool SHALL name the container holding the ports the proxy needs, and SHALL name the same one
however it is asked.

#### Scenario: A project that does not use the proxy is up

- **WHEN** the proxy is started while another environment publishes port 80 or 443
- **THEN** it is refused, and the container holding it is named

#### Scenario: One container holds 80 and another holds 443

- **WHEN** both ports are held by different containers
- **THEN** the one holding 80 is the one named

### Requirement: What is routed is what the router says

The tool SHALL report the proxy's routes as the proxy itself reports them.

#### Scenario: A router that never came up

- **WHEN** a container carries routing labels but the router failed
- **THEN** it is not reported as routed
