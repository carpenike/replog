// Convenience aliases over the OpenAPI-generated component schemas
// (src/api/generated.ts). Import from here rather than reaching into the deep
// `components['schemas'][...]` paths.
//
// Regenerate generated.ts with: `npm run gen:types` (see package.json).
//
// NOTE: the backend spec is Swagger 2.0, which emits every field as optional.
// The hand-written interfaces in ./types.ts are therefore still preferred where
// stricter (required-field) typing matters; adopt these generated aliases
// incrementally for shapes that were previously `unknown` or hand-mirrored.
import type { components } from './generated'

export type Schemas = components['schemas']

export type AthleteProgram = Schemas['api.AthleteProgram']
export type GenerationResponse = Schemas['api.GenerationResponse']
export type GenerationPreviewSchema = Schemas['api.GenerationPreview']
export type WorkoutSetSchema = Schemas['api.WorkoutSet']
export type WorkoutSetRequest = Schemas['api.WorkoutSetRequest']
export type WorkoutSetUpdateRequest = Schemas['api.WorkoutSetUpdateRequest']
