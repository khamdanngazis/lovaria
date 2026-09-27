-- name: ListDisabledThemes :many
SELECT theme_id FROM disabled_themes ORDER BY theme_id;

-- name: DisableTheme :exec
INSERT INTO disabled_themes (theme_id) VALUES ($1) ON CONFLICT (theme_id) DO NOTHING;

-- name: EnableTheme :exec
DELETE FROM disabled_themes WHERE theme_id = $1;
