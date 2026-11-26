// The surface the app imports. Everything under ./generated is written by
// `make codegen-ts` from spec/openapi.yaml; this file exists so components
// import from one place and the generated file's name can change without
// touching every call site.

export { ApiError } from './http'

export { createSnippet, deleteSnippet, getSnippet, listSnippets } from './generated/godoughAPI'

export type {
  CreateSnippetRequest,
  Error as ApiErrorBody,
  Snippet,
  ValidationError,
} from './generated/godoughAPI.schemas'
