package wallet

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
)

// ListCategoriesParams are the query parameters of GET /v1/api/categories.
type ListCategoriesParams struct {
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
	// CreatedAt filters by creation timestamp; up to two range conditions.
	CreatedAt []Filter
	// UpdatedAt filters by last sync timestamp; up to two range conditions.
	UpdatedAt []Filter
	// CustomCategory selects custom subcategories (true) or system categories
	// (false). Nil returns both.
	CustomCategory *bool
	// Cardinality filters by spending nature. A category with no cardinality
	// of its own inherits the base category's, so filtering by an inherited
	// value also matches the inheritors; "none" additionally matches
	// categories with nothing to inherit.
	Cardinality *Cardinality
	// Archived selects archived (true) or active (false) categories.
	Archived *bool
	// BudgetID returns only the categories a given budget tracks.
	BudgetID *string
	// SortBy orders the page. Only createdAt and updatedAt are sortable.
	SortBy *Sort
}

// Validate checks every documented constraint on the parameters.
func (p *ListCategoriesParams) Validate() error {
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
	probs.addErr(validateCardinality("cardinality", p.Cardinality))
	probs.addErr(validateSort("sortBy", p.SortBy, CommonSortFields))
	return probs.err()
}

func (p *ListCategoriesParams) query() url.Values {
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
	q.setFilters("createdAt", p.CreatedAt)
	q.setFilters("updatedAt", p.UpdatedAt)
	q.setBool("customCategory", p.CustomCategory)
	setEnum(q, "cardinality", p.Cardinality)
	q.setBool("archived", p.Archived)
	q.setString("budgetId", p.BudgetID)
	q.setSort("sortBy", p.SortBy)
	return q.values
}

// ListCategories returns one page of categories.
//
// Operation: getCategories. GET /v1/api/categories.
func (c *Client) ListCategories(ctx context.Context, params *ListCategoriesParams) (*CategoriesPage, error) {
	if err := c.validate(params); err != nil {
		return nil, err
	}
	out := &CategoriesPage{}
	err := c.do(ctx, request{
		op:     "getCategories",
		method: http.MethodGet,
		path:   "/categories",
		query:  params.query(),
	}, out)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// CreateSubcategoryRequest is the body of POST /v1/api/categories/custom.
type CreateSubcategoryRequest struct {
	// Name is the subcategory name. Required, at most 80 characters.
	Name string `json:"name"`
	// ParentID is the base category to nest under. Required.
	ParentID string `json:"parentId"`
	// Cardinality overrides the parent's spending nature.
	Cardinality *Cardinality `json:"cardinality,omitempty"`
	// Color is a universal colour name. It defaults to the parent's colour.
	Color *UniversalColor `json:"color,omitempty"`
}

// Validate checks every documented constraint on the body.
func (r *CreateSubcategoryRequest) Validate() error {
	if r == nil {
		return errorf("", "request body is required")
	}
	var p problems
	p.addErr(validateRequiredText("name", r.Name, MaxNameLength))
	p.addErr(validateRequiredID("parentId", r.ParentID))
	p.addErr(validateCardinality("cardinality", r.Cardinality))
	p.addErr(validateColor("color", r.Color))
	return p.err()
}

// CreateSubcategory creates a custom subcategory under a system category.
//
// Operation: createCategory. POST /v1/api/categories/custom. Answers 201.
func (c *Client) CreateSubcategory(ctx context.Context, body *CreateSubcategoryRequest) (*CreateCategoryResponse, error) {
	if err := c.validate(body); err != nil {
		return nil, err
	}
	out := &CreateCategoryResponse{}
	err := c.do(ctx, request{
		op:     "createCategory",
		method: http.MethodPost,
		path:   "/categories/custom",
		body:   body,
	}, out)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// ResettableCategoryField names a field PATCH /categories can restore to the
// base category's default.
type ResettableCategoryField string

// Fields the reset array accepts.
const (
	ResetName        ResettableCategoryField = "name"
	ResetCardinality ResettableCategoryField = "cardinality"
	// ResetColor restores the colour of the category's ENVELOPE GROUP, shared
	// by every category in that group — not the colour this category had
	// before. A colour set from a mobile or web app outside the 16 universal
	// names is therefore unrecoverable: read the current colour first if you
	// may want it back.
	ResetColor ResettableCategoryField = "color"
)

// AllResettableCategoryFields lists every field reset accepts.
var AllResettableCategoryFields = []ResettableCategoryField{ResetName, ResetCardinality, ResetColor}

// Valid reports whether the field can be reset.
func (f ResettableCategoryField) Valid() bool { return containsEnum(AllResettableCategoryFields, f) }

// PatchCategoryRequest is one item of PATCH /v1/api/categories.
type PatchCategoryRequest struct {
	// ID identifies the category to update, system or custom. Required.
	ID string `json:"id"`
	// Name is the new name; setting it also sets customName. It cannot be
	// combined with a reset of "name".
	Name *string `json:"name,omitempty"`
	// Cardinality is the new spending nature; "none" clears it.
	Cardinality *Cardinality `json:"cardinality,omitempty"`
	// Color is the new universal colour name.
	Color *UniversalColor `json:"color,omitempty"`
	// Reset restores fields to the base category's defaults. Base categories
	// only: a custom subcategory has no defaults to restore. A field cannot be
	// both set and reset in one call.
	Reset []ResettableCategoryField `json:"reset,omitempty"`
}

// Validate checks every documented constraint on one patch item.
func (r *PatchCategoryRequest) Validate() error {
	if r == nil {
		return errorf("", "patch item is required")
	}
	var p problems
	p.addErr(validateRequiredID("id", r.ID))
	if r.Name != nil {
		p.addErr(validateRequiredText("name", *r.Name, MaxNameLength))
	}
	p.addErr(validateCardinality("cardinality", r.Cardinality))
	p.addErr(validateColor("color", r.Color))

	conflicts := map[ResettableCategoryField]bool{
		ResetName:        r.Name != nil,
		ResetCardinality: r.Cardinality != nil,
		ResetColor:       r.Color != nil,
	}
	seen := map[ResettableCategoryField]bool{}
	for i, f := range r.Reset {
		at := fmt.Sprintf("reset[%d]", i)
		if !f.Valid() {
			p.add(at, "%q is not resettable; allowed: %s", f, enumNames(AllResettableCategoryFields))
			continue
		}
		if seen[f] {
			p.add(at, "%q is listed twice", f)
		}
		seen[f] = true
		if conflicts[f] {
			p.add(at, "%q cannot be reset and set in the same item", f)
		}
	}
	if r.Name == nil && r.Cardinality == nil && r.Color == nil && len(r.Reset) == 0 {
		p.add("", "no fields to update; set at least one field besides id")
	}
	return p.err()
}

// PatchCategoriesParams are the query parameters of PATCH /v1/api/categories.
type PatchCategoriesParams struct {
	// ReturnData controls whether each result carries the updated category.
	// The server defaults to true.
	ReturnData *bool
}

func (p *PatchCategoriesParams) query() url.Values {
	q := newQuery()
	if p == nil {
		return q.values
	}
	q.setBool("returnData", p.ReturnData)
	return q.values
}

type patchCategoriesBody []PatchCategoryRequest

func (b patchCategoriesBody) Validate() error {
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

// PatchCategories updates up to 10 categories in one call.
//
// Operation: patchCategories. PATCH /v1/api/categories. The response envelope
// is returned for 200, 207, 400 and 500 alike; inspect Summary and Results.
func (c *Client) PatchCategories(ctx context.Context, items []PatchCategoryRequest, params *PatchCategoriesParams) (*PatchCategoriesResponse, error) {
	if err := c.validate(patchCategoriesBody(items)); err != nil {
		return nil, err
	}
	out := &PatchCategoriesResponse{}
	err := c.do(ctx, request{
		op:     "patchCategories",
		method: http.MethodPatch,
		path:   "/categories",
		query:  params.query(),
		body:   items,
		batch:  true,
	}, out)
	if err != nil {
		return nil, err
	}
	return out, nil
}
