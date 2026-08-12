## ADDED Requirements

### Requirement: UI command

The CLI SHALL provide a `ui` command that starts the local portal server and
serves the embedded web app. It MUST allow choosing the bind address/port
(defaulting to a loopback address) and SHOULD print the URL to open. Like the
other commands it resolves the cluster via the standard kubeconfig.

#### Scenario: `ui` starts the local portal

- **WHEN** the user runs the `ui` command
- **THEN** a local HTTP server starts on the default loopback address and the command prints the URL to open

#### Scenario: Bind address is configurable

- **WHEN** the user runs the `ui` command with an explicit address/port
- **THEN** the portal is served on that address
