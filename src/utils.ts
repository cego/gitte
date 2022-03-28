import chalk from "chalk";

export function printLogs(projectNames: string[], logs: any[]) {
	// print the succesful logs
	for (const [i, projectName] of projectNames.entries()) {
		if (logs[i] instanceof Error) continue;
		console.log(chalk`┌─ {green {bold ${projectName}}}`);
		for (const [j, log] of logs[i].entries()) {
			console.log(`${j === logs[i].length - 1 ? "└" : "├"}─── ${log}`);
		}
	}
	// print the failed logs
	for (const [i, projectName] of projectNames.entries()) {
		if (!(logs[i] instanceof Error)) continue;
		console.log(chalk`┌─ {red {bold ${projectName}}}`);
		console.log(chalk`└─ {red ${logs[i].stack}}`);
	}
	if (logs.filter((l) => l instanceof Error).length > 0) {
		throw new Error("At least one git operation failed");
	}
}
