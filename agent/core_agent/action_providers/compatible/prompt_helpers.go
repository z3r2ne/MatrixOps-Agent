package compatible

import "matrixops.local/core_agent/streamtypes"

// ToActionPromptSchemas converts compatible action schemas for prompt injection.
func ToActionPromptSchemas(schemas []ActionSchema) []streamtypes.ActionPromptSchema {
	if len(schemas) == 0 {
		return nil
	}
	out := make([]streamtypes.ActionPromptSchema, 0, len(schemas))
	for _, schema := range schemas {
		out = append(out, streamtypes.ActionPromptSchema{
			ActionName:  schema.ActionName,
			Description: schema.Description,
			DataSchema:  schema.DataSchema,
		})
	}
	return out
}

// ResolveSessionActionSchemas returns action schemas for a compatible-mode session.
func ResolveSessionActionSchemas(enableCallToolReason bool, override []ActionSchema) []ActionSchema {
	if len(override) > 0 {
		return append([]ActionSchema(nil), override...)
	}
	return SessionActionSchemas(enableCallToolReason)
}
