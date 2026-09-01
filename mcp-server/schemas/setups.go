package schemas

type CreateSetupInput struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	AppIDs      []int  `json:"app_ids"`
}

type SetupIDInput struct {
	ID int `json:"id"`
}

type StartSetupInput struct {
	ID      int  `json:"id"`
	Confirm bool `json:"confirm"`
}

type UpdateSetupInput struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	AppIDs      []int  `json:"app_ids"`
	RemoveIcon  bool   `json:"remove_icon,omitempty"`
}

var setupProperties = map[string]any{
	"name":        map[string]any{"type": "string", "minLength": 1, "maxLength": 120},
	"description": map[string]any{"type": "string", "maxLength": 300},
	"app_ids": map[string]any{
		"type": "array", "minItems": 1, "uniqueItems": true,
		"items":       map[string]any{"type": "integer", "minimum": 1},
		"description": "Ordered saved desktop application IDs included in the setup.",
	},
}

var CreateSetupInputSchema = map[string]any{
	"type": "object", "properties": setupProperties,
	"required": []string{"name", "app_ids"}, "additionalProperties": false,
}

var SetupIDInputSchema = map[string]any{
	"type": "object",
	"properties": map[string]any{
		"id": map[string]any{
			"type":        "integer",
			"minimum":     1,
			"description": "Saved application setup ID.",
		},
	},
	"required":             []string{"id"},
	"additionalProperties": false,
}

var StartSetupInputSchema = map[string]any{
	"type": "object",
	"properties": map[string]any{
		"id": map[string]any{
			"type":        "integer",
			"minimum":     1,
			"description": "Saved application setup ID returned by list_setups.",
		},
		"confirm": map[string]any{
			"type":        "boolean",
			"const":       true,
			"description": "Must be true. Confirm only after the user explicitly asks to launch every application in the setup.",
		},
	},
	"required":             []string{"id", "confirm"},
	"additionalProperties": false,
}

var UpdateSetupInputSchema = map[string]any{
	"type": "object",
	"properties": map[string]any{
		"id":          map[string]any{"type": "integer", "minimum": 1},
		"name":        setupProperties["name"],
		"description": setupProperties["description"],
		"app_ids":     setupProperties["app_ids"],
		"remove_icon": map[string]any{"type": "boolean", "description": "Remove the existing setup icon. New icon uploads remain desktop-only."},
	},
	"required": []string{"id", "name", "app_ids"}, "additionalProperties": false,
}

type Setup struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	AppIDs      []int  `json:"app_ids"`
	IconURL     string `json:"icon_url,omitempty" jsonschema:"Desktop-local icon asset path when present."`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

type SetupsOutput struct {
	Setups []Setup `json:"setups" jsonschema:"Saved application setups."`
}

type SetupLaunchFailure struct {
	AppID   int    `json:"app_id"`
	AppName string `json:"app_name"`
	Error   string `json:"error"`
}

type SetupStartResult struct {
	Attempted int                  `json:"attempted"`
	Launched  int                  `json:"launched"`
	Failures  []SetupLaunchFailure `json:"failures"`
}
