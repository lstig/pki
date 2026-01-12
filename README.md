# PKI

Instructions and setup for air-gapped PKI.

## Pre-requisites

- Clean installation of the latest Raspberry Pi OS installed on a Raspberry Pi
  - Initial username: `admin`
  - SSH access enabled
- Ethernet connection (for initial bootstrap/setup)

## Setup

### Prepare the host

Determine the host's IP address and run the playbook.

> NOTE: The trailing `,` on `$HOST_IP` is important.

```
ansible-playbook -u admin -i $HOST_IP, playbook.yaml
```

### Disconnect the host from the network

Remove the Ethernet cable from the Raspberry Pi.

> [!CAUTION]
> **FROM THIS POINT FORWARD** the host should remain disconnected from the network.
> Connecting to a network could expose sensitive key material if the host or network are compromised.
