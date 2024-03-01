package riverjson

// Various concrete types used to marshal River values.
type (
	// jsonStatement is a statement within a River body.
	jsonStatement interface{ isStatement() }

	// A jsonBody is a collection of statements.
	jsonBody = []jsonStatement

	// jsonBlock represents a River block as JSON. jsonBlock is a jsonStatement.
	jsonBlock struct {
		Name  string          `json:"name"`
		Type  string          `json:"type"` // Always "block"
		Label string          `json:"label,omitempty"`
		Body  []jsonStatement `json:"body"`
	}

	// jsonAttr represents a River attribute as JSON. jsonAttr is a
	// jsonStatement.
	jsonAttr struct {
		Name  string    `json:"name"`
		Type  string    `json:"type"` // Always "attr"
		Value jsonValue `json:"value"`
	}

	// jsonValue represents a single River value as JSON.
	jsonValue struct {
		Type  string      `json:"type"`
		Value interface{} `json:"value"`
	}

	// jsonObjectField represents a field within a River object.
	jsonObjectField struct {
		Key   string      `json:"key"`
		Value interface{} `json:"value"`
	}
)

func (jsonBlock) isStatement() {}
func (jsonAttr) isStatement()  {}
