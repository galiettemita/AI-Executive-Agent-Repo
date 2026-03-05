module.exports = {
  root: true,
  parserOptions: {
    ecmaVersion: 2023,
    sourceType: 'module'
  },
  env: {
    es2023: true,
    node: true
  },
  extends: ['eslint:recommended'],
  ignorePatterns: ['dist/', 'node_modules/']
};
