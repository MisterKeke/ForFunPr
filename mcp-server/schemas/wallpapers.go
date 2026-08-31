package schemas

type WallpaperSelectionInput struct {
	Selection string `json:"selection"`
}
type UserWallpaperIDInput struct {
	ID string `json:"id"`
}

var WallpaperSelectionInputSchema = map[string]any{
	"type": "object", "properties": map[string]any{
		"selection": map[string]any{
			"type": "string", "description": "A built-in selection or custom wallpaper selection returned by get_wallpaper_settings.",
			"pattern": `^(builtin:(original|sandrone|hu-tao|skirk)|custom:[a-f0-9]{32})$`,
		},
	}, "required": []string{"selection"}, "additionalProperties": false,
}

var UserWallpaperIDInputSchema = map[string]any{
	"type": "object", "properties": map[string]any{
		"id": map[string]any{"type": "string", "pattern": `^[a-f0-9]{32}$`, "description": "User wallpaper ID from get_wallpaper_settings."},
	}, "required": []string{"id"}, "additionalProperties": false,
}

type UserWallpaper struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
	MIMEType    string `json:"mime_type"`
	ByteSize    int64  `json:"byte_size"`
	URL         string `json:"url" jsonschema:"Desktop-local wallpaper asset path."`
}

type WallpaperSettings struct {
	Selected          string          `json:"selected"`
	SelectionSaved    bool            `json:"selection_saved"`
	BuiltinSelections []string        `json:"builtin_selections"`
	UserWallpapers    []UserWallpaper `json:"user_wallpapers"`
}
