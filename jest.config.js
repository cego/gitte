/** @type {import('ts-jest/dist/types').InitialOptionsTsJest} */
module.exports = {
  preset: 'ts-jest',
  testEnvironment: 'node',
  collectCoverageFrom: [
    "src/**/*.js"
  ],
  coverageReporters: [
    "lcov",
    "json-summary",
    "text-summary"
  ]
};

