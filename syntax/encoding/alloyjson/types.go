package alloyjson

// Various concrete types used to marshal Alloy values.
type (
	// jsonStatement is a statement within an Alloy body.
	jsonStatement interface{ isStatement() }

	// A jsonBody is a collection of statements.
	jsonBody = []jsonStatement

	// jsonBlock represents an Alloy block as JSON. jsonBlock is a jsonStatement.
	jsonBlock struct {
		Name  string          `json:"name"`
		Type  string          `json:"type"` // Always "block"
		Label string          `json:"label,omitempty"`
		Body  []jsonStatement `json:"body"`
	}

	// jsonAttr represents an Alloy attribute as JSON. jsonAttr is a
	// jsonStatement.
	jsonAttr struct {
		Name  string    `json:"name"`
		Type  string    `json:"type"` // Always "attr"
		Value jsonValue `json:"value"`
	}

	// jsonValue represents a single Alloy value as JSON.
	jsonValue struct {
		Type  string `json:"type"`
		Value any    `json:"value"`
	}

	// jsonObjectField represents a field within an Alloy object.
	jsonObjectField struct {
		Key   string `json:"key"`
		Value any    `json:"value"`
	}
)

func (jsonBlock) isStatement() {}
func (jsonAttr) isStatement()  {}
