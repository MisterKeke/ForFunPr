package schemas

type RenameDesktopAppInput struct {
	ID          int    `json:"id"`
	DisplayName string `json:"display_name"`
}

var RenameDesktopAppInputSchema = map[string]any{
	"type": "object",
	"properties": map[string]any{
		"id":           map[string]any{"type": "integer", "minimum": 1, "description": "Saved desktop application ID."},
		"display_name": map[string]any{"type": "string", "minLength": 1, "maxLength": 120, "description": "Replacement display name."},
	},
	"required": []string{"id", "display_name"}, "additionalProperties": false,
}

type DesktopApp struct {
	ID          int    `json:"id" jsonschema:"Saved desktop application ID."`
	DisplayName string `json:"display_name" jsonschema:"User-visible application name."`
	IconURL     string `json:"icon_url,omitempty" jsonschema:"Desktop-local icon asset path when present."`
	Available   bool   `json:"available" jsonschema:"Whether the saved executable currently exists."`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

type DesktopAppsOutput struct {
	Applications []DesktopApp `json:"applications" jsonschema:"Saved applications; executable filesystem paths are intentionally omitted."`
}
