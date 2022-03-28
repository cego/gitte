module.exports = {
	preset: "ts-jest",
	testMatch: [
		"**/*.test.ts"
	],
	coverageReporters: [
		"lcov",
		"json-summary",
		"text-summary"
	]
};
