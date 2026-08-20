package deploy

import "embed"

// Assets embeds the deploy/ YAML manifests so e2e tests can apply the same
// files used for local installation. The OTE test binary has no repo checkout.
//
// Tests apply a named subset of these files via resourceapply. Deployment
// images, --serve-artifacts-in-http, and Trace logLevel are e2e-only mutations
// applied in Go — they are not baked into these YAML files.
//
//go:embed *.yaml
var Assets embed.FS
