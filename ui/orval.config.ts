import { defineConfig } from 'orval'

// Generates the request/response types and one function per operation from the
// contract the Go server is built from. The output is committed: the web image
// builds from ui/ alone and cannot see a spec at the repository root, and a
// drift gate needs a committed copy to diff against. `make codegen-check` is
// that gate.
export default defineConfig({
  godough: {
    input: { target: '../spec/openapi.yaml' },
    output: {
      target: './src/api/generated',
      client: 'fetch',
      mode: 'split',
      mock: false,
      clean: true,
      // Without this every generated function resolves to the { data, status,
      // headers } wrapper, and the error branch is a cast rather than a type.
      override: {
        mutator: { path: './src/api/http.ts', name: 'request' },
        fetch: { includeHttpResponseReturnType: false },
      },
    },
  },
})
