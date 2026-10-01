package model

type Module struct {
	ID                   string        `json:"id"`
	ParentModuleID       *string       `json:"-"`
	CreatedByComponentID *string       `json:"-"`
	ChildModuleIDs       []string      `json:"-"`
	ComponentIDs         []string      `json:"-"`
	RuntimeGraph         *RuntimeGraph `json:"-"`
}
