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

export async function wrapEntryPromiseWithKey<TArgs extends any[], TReturn>(arg: [string, TArgs], fn: (...args: TArgs) => TReturn): Promise<{key: string, res: T2}> {
	const res = await fn(...arg[1]);
	return { key: arg[0], res };
}

export function waitingOnToString(waitingOn: string[]): string {
	// max 40 chars
	let str = "";
	for(let [i,waitingOnStr] of waitingOn.entries()) {
		if(i !== 0 && str.length + waitingOnStr.length > 40){
			return `${str} and ${waitingOn.length - i} more`;
		}
		str += waitingOnStr;
	}
	return str;

}
