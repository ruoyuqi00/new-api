-- Add operator-friendly Claude Opus 4.7 aliases for Windsurf-backed accounts.
--
-- WindsurfAPI exposes Opus 4.7 with dash-separated effort keys such as
-- claude-opus-4-7-medium. Users commonly try the official/dotted family name,
-- so map those aliases to the existing Windsurf routing keys.

UPDATE accounts
SET credentials = jsonb_set(
    jsonb_set(
      jsonb_set(
        jsonb_set(
          jsonb_set(
            jsonb_set(
              jsonb_set(
                jsonb_set(
                  credentials,
                  '{model_mapping,claude-opus-4.7}',
                  '"claude-opus-4-7-medium"'::jsonb,
                  true
                ),
                '{model_mapping,claude-opus-4-7}',
                '"claude-opus-4-7-medium"'::jsonb,
                true
              ),
              '{model_mapping,claude-opus-4.7-low}',
              '"claude-opus-4-7-low"'::jsonb,
              true
            ),
            '{model_mapping,claude-opus-4.7-medium}',
            '"claude-opus-4-7-medium"'::jsonb,
            true
          ),
          '{model_mapping,claude-opus-4.7-high}',
          '"claude-opus-4-7-high"'::jsonb,
          true
        ),
        '{model_mapping,claude-opus-4.7-max}',
        '"claude-opus-4-7-max"'::jsonb,
        true
      ),
      '{model_mapping,claude-opus-4.7-xhigh}',
      '"claude-opus-4-7-xhigh"'::jsonb,
      true
    ),
    '{model_mapping,claude-opus-4.7-medium-thinking}',
    '"claude-opus-4-7-medium-thinking"'::jsonb,
    true
  ),
  updated_at = NOW()
WHERE deleted_at IS NULL
  AND platform = 'anthropic'
  AND type = 'apikey'
  AND credentials->>'base_url' LIKE '%windsurf-api%'
  AND credentials->'model_mapping' ? 'claude-opus-4-7-medium';
