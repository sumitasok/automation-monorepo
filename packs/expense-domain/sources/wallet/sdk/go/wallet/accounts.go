package wallet

import (
	"context"
	"net/http"
	"net/url"
)

// ListAccountsParams are the query parameters of GET /v1/api/accounts.
// Every field is optional; a nil or empty field is simply not sent.
type ListAccountsParams struct {
	// Limit is the page size, 1-200. The server defaults to 30.
	Limit *int
	// Offset skips items for pagination.
	Offset *int
	// AgentHints asks for structured hints alongside the data.
	AgentHints *bool
	// WithTotal asks for a total count, at the cost of an extra query.
	WithTotal *bool
	// IDs filters by identifier, up to 30 of them.
	IDs []string
	// Name filters by name and requires a text prefix.
	Name *Filter
	// AccountType filters by type, case-insensitively.
	AccountType *AccountType
	// CurrencyCode filters by ISO 4217 code, case-insensitively.
	CurrencyCode *string
	// CreatedAt filters by creation timestamp; up to two range conditions.
	CreatedAt []Filter
	// UpdatedAt filters by last sync timestamp; up to two range conditions.
	UpdatedAt []Filter
	// Archived selects archived (true) or active (false) accounts. Nil returns
	// both.
	Archived *bool
	// BudgetID returns only the accounts a given budget tracks.
	BudgetID *string
	// SortBy orders the page. Only createdAt and updatedAt are sortable.
	SortBy *Sort
}

// Validate checks every documented constraint on the parameters.
func (p *ListAccountsParams) Validate() error {
	if p == nil {
		return nil
	}
	var probs problems
	probs.addErr(validateLimit(p.Limit))
	probs.addErr(validateOffset(p.Offset))
	probs.addErr(validateIDList("id", p.IDs, MaxIDFilterValues))
	if p.Name != nil {
		probs.addErr(validateTextFilter("name", *p.Name))
	}
	probs.addErr(validateRangeFilters("createdAt", p.CreatedAt))
	probs.addErr(validateRangeFilters("updatedAt", p.UpdatedAt))
	if p.AccountType != nil && !p.AccountType.Valid() {
		probs.add("accountType", "%q is not an account type; allowed: %s", *p.AccountType, enumNames(AllAccountTypes))
	}
	if p.CurrencyCode != nil {
		probs.addErr(validateCurrencyCode("currencyCode", *p.CurrencyCode))
	}
	probs.addErr(validateSort("sortBy", p.SortBy, CommonSortFields))
	return probs.err()
}

func (p *ListAccountsParams) query() url.Values {
	q := newQuery()
	if p == nil {
		return q.values
	}
	q.setInt("limit", p.Limit)
	q.setInt("offset", p.Offset)
	q.setBool("agentHints", p.AgentHints)
	q.setBool("withTotal", p.WithTotal)
	q.setCSV("id", p.IDs)
	q.setFilter("name", p.Name)
	setEnum(q, "accountType", p.AccountType)
	q.setString("currencyCode", p.CurrencyCode)
	q.setFilters("createdAt", p.CreatedAt)
	q.setFilters("updatedAt", p.UpdatedAt)
	q.setBool("archived", p.Archived)
	q.setString("budgetId", p.BudgetID)
	q.setSort("sortBy", p.SortBy)
	return q.values
}

// ListAccounts returns one page of financial accounts.
//
// Operation: getAccounts. GET /v1/api/accounts.
func (c *Client) ListAccounts(ctx context.Context, params *ListAccountsParams) (*AccountsPage, error) {
	if err := c.validate(params); err != nil {
		return nil, err
	}
	out := &AccountsPage{}
	err := c.do(ctx, request{
		op:     "getAccounts",
		method: http.MethodGet,
		path:   "/accounts",
		query:  params.query(),
	}, out)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// CreateAccountRequest is the body of POST /v1/api/accounts.
type CreateAccountRequest struct {
	// Name is the display name. Required, at most 80 characters.
	Name string `json:"name"`
	// AccountType is the kind of account. Required, and must be one of the
	// five creatable types.
	AccountType AccountType `json:"accountType"`
	// CurrencyCode is the 3-letter fiat currency. Required.
	CurrencyCode string `json:"currencyCode"`
	// InitialBalance is the opening balance. Required — the API rejects the
	// body without it, so send 0 for an empty account. It is write-only: read
	// it back as Account.Balance.Initial.
	InitialBalance float64 `json:"initialBalance"`
	// BankAccountNumber is the account number or IBAN, at most 80 characters.
	BankAccountNumber *string `json:"bankAccountNumber,omitempty"`
	// Color is a universal colour name. A random one is assigned if omitted.
	Color *UniversalColor `json:"color,omitempty"`
	// ExcludeFromStats keeps the account out of aggregate statistics.
	ExcludeFromStats *bool `json:"excludeFromStats,omitempty"`
}

// Validate checks every documented constraint on the body.
func (r *CreateAccountRequest) Validate() error {
	if r == nil {
		return errorf("", "request body is required")
	}
	var p problems
	p.addErr(validateRequiredText("name", r.Name, MaxNameLength))
	switch {
	case r.AccountType == "":
		p.add("accountType", "is required")
	case !r.AccountType.Creatable():
		p.add("accountType", "%q cannot be created over REST; allowed: %s",
			r.AccountType, enumNames(CreatableAccountTypes))
	}
	p.addErr(validateCurrencyCode("currencyCode", r.CurrencyCode))
	if r.BankAccountNumber != nil {
		p.addErr(validateMaxLen("bankAccountNumber", *r.BankAccountNumber, MaxBankAccountNumberLength))
	}
	p.addErr(validateColor("color", r.Color))
	return p.err()
}

// CreateAccount creates a financial account.
//
// Operation: createAccount. POST /v1/api/accounts. Answers 201 on success.
func (c *Client) CreateAccount(ctx context.Context, body *CreateAccountRequest) (*CreateAccountResponse, error) {
	if err := c.validate(body); err != nil {
		return nil, err
	}
	out := &CreateAccountResponse{}
	err := c.do(ctx, request{
		op:     "createAccount",
		method: http.MethodPost,
		path:   "/accounts",
		body:   body,
	}, out)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// PatchAccountItem is one item of PATCH /v1/api/accounts. Every field except
// ID is optional; an omitted field is left unchanged.
type PatchAccountItem struct {
	// ID identifies the account to update. Required.
	ID string `json:"id"`
	// Name is the new display name, at most 80 characters.
	Name *string `json:"name,omitempty"`
	// Archived sets or clears the archived flag.
	Archived *bool `json:"archived,omitempty"`
	// BankAccountNumber is the new account number or IBAN. An empty string
	// clears the value.
	BankAccountNumber *string `json:"bankAccountNumber,omitempty"`
	// Color is the new universal colour name.
	Color *UniversalColor `json:"color,omitempty"`
	// ExcludeFromStats sets or clears the statistics exclusion.
	ExcludeFromStats *bool `json:"excludeFromStats,omitempty"`
	// InitialBalance is the new opening balance. Write-only: read it back as
	// Account.Balance.Initial.
	InitialBalance *float64 `json:"initialBalance,omitempty"`
}

// Validate checks every documented constraint on the item.
func (i *PatchAccountItem) Validate() error {
	if i == nil {
		return errorf("", "patch item is required")
	}
	var p problems
	p.addErr(validateRequiredID("id", i.ID))
	if i.Name != nil {
		p.addErr(validateRequiredText("name", *i.Name, MaxNameLength))
	}
	if i.BankAccountNumber != nil {
		// An empty string is meaningful here: it clears the value.
		p.addErr(validateMaxLen("bankAccountNumber", *i.BankAccountNumber, MaxBankAccountNumberLength))
	}
	p.addErr(validateColor("color", i.Color))
	if i.Name == nil && i.Archived == nil && i.BankAccountNumber == nil &&
		i.Color == nil && i.ExcludeFromStats == nil && i.InitialBalance == nil {
		p.add("", "no fields to update; set at least one field besides id")
	}
	return p.err()
}

// PatchAccountsParams are the query parameters of PATCH /v1/api/accounts.
type PatchAccountsParams struct {
	// ReturnData controls whether each result carries the updated account.
	// The server defaults to true.
	ReturnData *bool
}

func (p *PatchAccountsParams) query() url.Values {
	q := newQuery()
	if p == nil {
		return q.values
	}
	q.setBool("returnData", p.ReturnData)
	return q.values
}

// patchAccountsBody wraps the item slice so it can carry a Validate method.
type patchAccountsBody []PatchAccountItem

func (b patchAccountsBody) Validate() error {
	var p problems
	p.addErr(validateBatchSize("items", len(b), MaxPatchItems))
	seen := map[string]int{}
	for i := range b {
		p.prefixed(itemPath(i), b[i].Validate())
		if id := b[i].ID; id != "" {
			if first, dup := seen[id]; dup {
				p.add(itemPath(i)+".id", "duplicates items[%d] (%q); the API applies both in order", first, id)
			} else {
				seen[id] = i
			}
		}
	}
	return p.err()
}

// PatchAccounts updates up to 10 accounts in one call.
//
// Operation: patchAccounts. PATCH /v1/api/accounts. The response envelope is
// returned for 200, 207, 400 and 500 alike; inspect Summary and Results.
func (c *Client) PatchAccounts(ctx context.Context, items []PatchAccountItem, params *PatchAccountsParams) (*PatchAccountsResponse, error) {
	if err := c.validate(patchAccountsBody(items)); err != nil {
		return nil, err
	}
	out := &PatchAccountsResponse{}
	err := c.do(ctx, request{
		op:     "patchAccounts",
		method: http.MethodPatch,
		path:   "/accounts",
		query:  params.query(),
		body:   items,
		batch:  true,
	}, out)
	if err != nil {
		return nil, err
	}
	return out, nil
}
