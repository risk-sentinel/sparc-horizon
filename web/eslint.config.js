// The HUD's lint gate (S1-7). Type-aware, so it reads tsconfig.json.
import js from "@eslint/js";
import tseslint from "typescript-eslint";

export default tseslint.config(
  { ignores: ["dist/", "node_modules/", "test-results/", "playwright-report/", "eslint.config.js"] },
  js.configs.recommended,
  ...tseslint.configs.strictTypeChecked,
  {
    languageOptions: { parserOptions: { projectService: true, tsconfigRootDir: import.meta.dirname } },
    rules: {
      // Every string from a document is untrusted: build nodes, never parse markup.
      "no-restricted-properties": [
        "error",
        { property: "innerHTML", message: "Use textContent or el(); document strings are untrusted." },
        { property: "outerHTML", message: "Use textContent or el(); document strings are untrusted." },
        { property: "insertAdjacentHTML", message: "Use el(); document strings are untrusted." },
      ],
      "no-eval": "error",
      "no-implied-eval": "error",
      "@typescript-eslint/restrict-template-expressions": ["error", { allowNumber: true }],
      // node:test's test() and describe() return promises the runner awaits.
      "@typescript-eslint/no-floating-promises": [
        "error",
        { allowForKnownSafeCalls: [{ from: "package", package: "node:test", name: ["test", "describe"] }] },
      ],
    },
  },
);
