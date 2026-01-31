import stylistic from "@stylistic/eslint-plugin";
import tseslint from "typescript-eslint";

export default tseslint.config({
  files: ["**/*.ts", "**/*.tsx", "**/*.js", "**/*.jsx"],
  languageOptions: {
    parser: tseslint.parser,
    parserOptions: {
      sourceType: "module",
      ecmaVersion: "latest",
    },
  },
  plugins: {
    "@stylistic": stylistic,
  },
  rules: {
    "@stylistic/brace-style": ["error", "allman", { allowSingleLine: true }],
    "@stylistic/indent": ["error", 4],
    "@stylistic/semi": ["error", "always"],
    "@stylistic/object-curly-spacing": ["error", "always"],
  },
});
