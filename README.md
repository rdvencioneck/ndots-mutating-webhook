# ndots-mutating-webhook
A simple mutating webhook that sets ndots in DNSConfig into new pods
Based on the example from https://github.com/trstringer/kubernetes-mutating-webhook

It keeps other dnsConfig definitions, and will only include ndots config if not previously defined during pod creation.

# Chart

There's also a chart included in this repo that deploys everything needed.
Make sure to check [values.yaml](charts/ndots-injector-mutating-webhook/values.yaml)

## Requirement

- [Cert-manager](https://cert-manager.io/) is required for the cert automation.

## Deployment Examples

Check the [examples folder](examples) for deployment options