// Package wallet is a complete, dependency-free client for the BudgetBakers
// Wallet REST API (https://rest.budgetbakers.com/wallet), OpenAPI 3.1 spec
// version 2.0.0.
//
// The package covers every documented operation, models every documented
// schema, validates every documented constraint before a request leaves the
// process, and surfaces every documented failure mode as a typed error.
//
// # Authentication
//
// All endpoints take a bearer token minted for the Wallet flavour of the API
// (a Board token on a Wallet path yields 403 — see ErrForbidden).
//
//	c, err := wallet.New(wallet.Config{Token: os.Getenv("WALLET_API_TOKEN")})
//	if err != nil { return err }
//	page, err := c.ListAccounts(ctx, &wallet.ListAccountsParams{Limit: wallet.Int(50)})
//
// # Batch endpoints
//
// POST /records, and every PATCH and DELETE, are batch endpoints. They answer
// 200 (all succeeded), 207 (mixed), 400 (all failed with client errors) and
// 500 (all failed with server errors) with the SAME response envelope. This
// SDK therefore returns the parsed envelope — not an error — for all four
// statuses, and the caller inspects Summary and Results. A 400 that is a
// plain ValidationError (malformed body, batch too large) is still returned
// as an error; the two are told apart by the shape of the body.
//
// # Errors
//
// Every non-2xx that is not a batch envelope becomes a *Error carrying the
// status code, the server's message, the raw body and the response metadata.
// Match on the sentinels with errors.Is: ErrValidation, ErrUnauthorized,
// ErrForbidden, ErrNotFound, ErrRateLimited, ErrServer, ErrUnexpectedStatus,
// ErrDecode and ErrTransport. Client-side validation failures never reach the
// network and match ErrInvalidRequest.
package wallet
