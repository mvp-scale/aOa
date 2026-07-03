# Vendored viewer bundle — provenance & regeneration

`bundle.js.gz` is the pre-compressed ESM bundle the viewer loads at
`/arch/vendor/bundle.js`. It is stored gzipped to stay under the repo's 1MB
file hook; the handler serves it with `Content-Encoding: gzip` (or
decompressed for non-gzip clients).

## Exact contents

| Package | Version |
|---|---|
| react | 18.3.1 |
| react-dom | 18.3.1 |
| @xyflow/react | 12.3.5 |
| elkjs | 0.11.1 |
| htm | 3.1.1 |

`xyflow.css` is `@xyflow/react/dist/style.css` from the same version.

## Regenerate

```bash
cd $(mktemp -d)
cat > entry.js <<'EOF'
export * as React from "react";
export * as ReactDOM from "react-dom/client";
export * as ReactFlow from "@xyflow/react";
export { default as ELK } from "elkjs/lib/elk.bundled.js";
export { default as htm } from "htm";
EOF
npm install react@18.3.1 react-dom@18.3.1 @xyflow/react@12.3.5 elkjs@0.11.1 htm@3.1.1
npx esbuild entry.js --bundle --format=esm --platform=browser --minify --outfile=bundle.js
gzip -9 bundle.js
cp bundle.js.gz {repo}/internal/adapters/web/static/arch/vendor/
cp node_modules/@xyflow/react/dist/style.css {repo}/internal/adapters/web/static/arch/vendor/xyflow.css
```

Budgets (asserted by `TestT16BundleBudget`): raw ≤ 2.2MB, gz ≤ 650KB.
Bump versions here AND in the T16 test's error message when upgrading.
