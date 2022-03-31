import chalk from "chalk";
import cliProgress from "cli-progress";

export async function wrapEntryPromiseWithKey<TArg, TReturn>(
	arg: [string, TArg],
	fn: (arg: TArg) => TReturn,
): Promise<{ key: string; res: TReturn }> {
	const res = await fn(arg[1]);
	return { key: arg[0], res };
}

export function waitingOnToString(waitingOn: string[]): string {
	if (waitingOn.length === 0) return "Finished all tasks";
	let str = "";
	for (const [i, waitingOnStr] of waitingOn.entries()) {
		if (i !== 0 && str.length + waitingOnStr.length > 40) {
			return `${str} and ${waitingOn.length - i} more`;
		}
		str += `${str ? ", " : ""}${waitingOnStr}`;
	}
	return str;
}

export async function applyPromiseToEntriesWithProgressBar<TArg, TReturn>(
	label: string,
	entries: [string, TArg][],
	fn: (arg: TArg) => TReturn,
): Promise<Promise<TReturn>[]> {
	const promises = entries.map(([key, value]) => wrapEntryPromiseWithKey([key, value], fn));
	const waitingOn = entries.map(([key]) => key);

	const progressBar = getProgressBar(label);
	progressBar.start(promises.length, 0, { status: waitingOnToString(waitingOn) });

	const result = await Promise.all(
		promises.map((p) =>
			p
				.then((res) => {
					waitingOn.splice(waitingOn.indexOf(res.key), 1);
					progressBar.increment({ status: waitingOnToString(waitingOn) });
					return res.res;
				})
				.catch((e) => e),
		),
	);
	progressBar.stop();
	return result;
}

export function getProgressBar(label: string) {
	return new cliProgress.SingleBar(
		{
			format: chalk`\{bar\} \{value\}/\{total\} | ${label}: {cyan \{status\}} `,
		},
		cliProgress.Presets.shades_classic,
	);
}

export async function applyPromiseToEntriesWithProgressBarSync<TArg, TReturn>(
	label: string,
	entries: [string, TArg][],
	fn: (arg: TArg) => TReturn,
): Promise<TReturn[]> {
	const progressBar = getProgressBar(label);

	progressBar.start(entries.length, 0);

	const result = [] as TReturn[];
	for (const [key, value] of entries) {
		progressBar.update({ status: key });
		result.push(await fn(value));
		progressBar.increment();
	}

	progressBar.update({ status: "Finished all tasks" });
	progressBar.stop();
	return result;
}
