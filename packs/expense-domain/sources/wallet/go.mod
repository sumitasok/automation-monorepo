module github.com/sumitasok/sa.automation.wallet

go 1.21

// Pure standard library — no external dependencies. Builds offline; no go.sum.

// The UI decodes records with the SDK's own Record type so the dashboard and
// the API client cannot drift apart. Local module, so still no go.sum.
require github.com/sumitasok/sa.automation.wallet/sdk/go v0.0.0

replace github.com/sumitasok/sa.automation.wallet/sdk/go => ./sdk/go
