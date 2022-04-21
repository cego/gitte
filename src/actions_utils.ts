import chalk from "chalk";
import { Config, ProjectAction } from "./types/config";
import { ChildProcessOutput, GroupKey } from "./types/utils";
import { logActionOutput, searchOutputForHints } from "./search_output";
import { printHeader } from "./utils";
import { getProgressBar, waitingOnToString } from "./progress";
import { SingleBar } from "cli-progress";
import fs from "fs-extra";
import path from "path";
import { Writable } from "stream";
import ansiEscapes from "ansi-escapes";
import ON_DEATH from "death";
import { actions } from "./actions";
import assert, { AssertionError } from "assert";
import { getProjectDirFromRemote } from "./project";

class BufferStreamWithTty extends Writable {
	isTTY = true;
}

export class ActionOutputPrinter {
	maxLines = 10;
	lastFewLines: { out: string; project: string }[] = [];
	progressBar?: SingleBar;
	actionsToRun: string[];
	groupsToRun: string[];
	projectsToRun: string[];
	config: Config;
	waitingOn = [] as string[];
	termBuffer = "";
	bufferStream?: BufferStreamWithTty;
	// Holds information on what commands have been run in which paths. Used to deduplicate.
	alreadyRunDirCmdPairs = new Map<string, Set<string>>();

	constructor(cfg: Config, actionToRun: string, groupToRun: string, projectToRun: string) {
		// First parse actionToRun, groupToRun and projectToRun

		this.config = cfg;

		const [actionsToRun, groupsToRun, projectsToRun] = this.parseRunKeys(actionToRun, groupToRun, projectToRun);
		this.actionsToRun = actionsToRun;
		this.groupsToRun = groupsToRun;
		this.projectsToRun = projectsToRun;
	}

	addToBufferStream = (chunk: string) => {
		this.termBuffer += chunk;
	};

	printOutputLines = () => {
		let toWrite = "";
		toWrite += ansiEscapes.cursorUp(this.maxLines + 1);

		// Avoid printing multiple iterations of progress bar..
		const splitString = "\u001b[0K\u001b[1G";
		const splittedTermbuffer = this.termBuffer.split(splitString);
		this.termBuffer = splitString + splittedTermbuffer[splittedTermbuffer.length - 1];

		toWrite += this.termBuffer;
		toWrite += ansiEscapes.cursorDown(1);
		const width = process.stdout.columns;
		for (let i = 0; i < this.maxLines; i++) {
			toWrite += ansiEscapes.cursorDown(1) + ansiEscapes.cursorLeft + ansiEscapes.eraseLine;
			// get terminal width
			if (this.lastFewLines[i]) {
				const maxWidth = Math.max(width - (this.lastFewLines[i].project.length + 3), 0);
				toWrite += chalk`{inverse  ${this.lastFewLines[i].project} } {gray ${this.lastFewLines[i].out.slice(
					0,
					maxWidth,
				)}}`;
			}
		}
		process.stdout.write(toWrite);
	};

	handleLogOutput = (str: string, projectName: string) => {
		// Only print "printable" characters
		str = str.replace(/[\p{Cc}\p{Cf}\p{Cs}]+/gu, "");

		const lines = str
			.split("\n")
			.map((splitted) => splitted.replace(/\r/g, ""))
			.filter((splitted) => splitted.length);
		lines.forEach((line) => {
			this.lastFewLines.push({ out: line, project: projectName });
		});

		while (this.lastFewLines.length > this.maxLines) {
			this.lastFewLines.shift();
		}
	};

	getWritableStream = (name: string) => {
		const handle = this.handleLogOutput;
		return new Writable({
			write(chunk, _, callback) {
				handle(chunk.toString(), name);
				callback();
			},
		});
	};

	clearOutputLines = async () => {
		process.stdout.write(
			ansiEscapes.cursorShow +
				ansiEscapes.cursorUp(this.maxLines) +
				ansiEscapes.cursorLeft +
				ansiEscapes.eraseDown +
				ansiEscapes.cursorDown(1),
		);
	};
	prepareOutputLines = () => {
		const showCursor = () => {
			process.stdout.write(ansiEscapes.cursorShow);
		};
		process.on("exit", showCursor);
		ON_DEATH(showCursor);
		process.stdout.write(ansiEscapes.cursorHide + Array(this.maxLines + 2).join("\n"));
	};

	beganTask = (keys: GroupKey): boolean => {
		const project = this.config.projects[keys.project];
		const action = project.actions[keys.action];
		const group = action.groups[keys.group] ?? action.groups["*"];
		const cmd = group.join(" ");
		const dir = getProjectDirFromRemote(this.config.cwd, this.config.projects[keys.project].remote);
		if (this.alreadyRunDirCmdPairs.get(dir)?.has(cmd)) {
			this.progressBar?.increment();
			return false;
		}

		if (this.alreadyRunDirCmdPairs.has(dir)) {
			// eslint-disable-next-line @typescript-eslint/no-non-null-assertion
			this.alreadyRunDirCmdPairs.get(dir)!.add(cmd);
		} else {
			this.alreadyRunDirCmdPairs.set(dir, new Set([cmd]));
		}

		this.waitingOn.push(keys.project);
		this.progressBar?.update({ status: waitingOnToString(this.waitingOn) });

		return true;
	};

	finishedTask = (project: string) => {
		this.waitingOn = this.waitingOn.filter((p) => p !== project);
		this.progressBar?.increment({ status: waitingOnToString(this.waitingOn) });
	};

	/**
	 * Called by action runner, should not be called anywhere else.
	 * @param actionsToRun
	 */
	init = (actionsToRun: (GroupKey & ProjectAction)[]): void => {
		this.progressBar?.start(actionsToRun.length, 0, { status: waitingOnToString([]) });
	};

	run = async (): Promise<void> => {
		for (const action of this.actionsToRun) {
			let catched: AssertionError | undefined;
			for (const group of this.groupsToRun) {
				try {
					await this.runActionUtils(action, group);
				} catch (e) {
					if (e instanceof AssertionError) {
						// catch it but finish other groups..
						catched = e;
					} else {
						throw e;
					}
				}
			}
			if (catched) {
				throw catched;
			}
		}
	};

	runActionUtils = async (actionToRun: string, groupToRun: string): Promise<void> => {
		const addToBufferStream = this.addToBufferStream;
		this.bufferStream = new BufferStreamWithTty({
			write(chunk, _, callback) {
				addToBufferStream(chunk.toString());
				callback();
			},
		});
		this.progressBar = getProgressBar(`Running ${actionToRun} ${groupToRun}`, this.bufferStream);

		printHeader(`Running action ${actionToRun} on group ${groupToRun}`);
		this.prepareOutputLines();
		// every 100ms, print output
		const interval = setInterval(() => {
			this.printOutputLines();
		}, 100);
		const stdoutBuffer: (GroupKey & ChildProcessOutput)[] = await actions(
			this.config,
			actionToRun,
			groupToRun,
			this.projectsToRun,
			this,
		);
		clearInterval(interval);
		this.progressBar.update({ status: waitingOnToString(null) });
		this.progressBar.stop();
		// final flush
		this.printOutputLines();
		await this.clearOutputLines();
		const isError = logActionOutput(stdoutBuffer);
		if (this.config.searchFor) searchOutputForHints(this.config, stdoutBuffer);
		if (stdoutBuffer.length === 0) {
			console.log(chalk`{yellow No actions was found for the provided action, group and project.}`);
		}
		this.termBuffer = "";

		await this.stashLogsToFile(stdoutBuffer);

		assert(!isError, "At least one action failed");
	};

	stashLogsToFile = async (logs: (GroupKey & ChildProcessOutput)[]) => {
		const logsFolderPath = path.join(this.config.cwd, "logs");
		if (!(await fs.pathExists(logsFolderPath))) {
			await fs.mkdir(logsFolderPath);
		}

		for (const log of logs) {
			const logsFilePath = path.join(logsFolderPath, `${log.action}-${log.group}-${log.project}.log`);
			const output = [];
			output.push(...(log.stdout?.split("\n").map((line) => `[stdout] ${line.trim()}`) ?? []));
			output.push(...(log.stderr?.split("\n").map((line) => `[stderr] ${line.trim()}`) ?? []));
			output.push(
				`[exitCode] ${log.cmd?.join(" ")} exited with ${log.exitCode} in ${log.dir} at ${new Date().toISOString()}`,
			);
			await fs.writeFile(logsFilePath, output.join("\n"));
		}
	};

	parseRunKeys = (actionToRun: string, groupToRun: string, projectToRun: string): [string[], string[], string[]] => {
		const delimiter = "+";
		let actionsToRun = actionToRun.split(delimiter);
		// If '*' is in actionsToRun, then we run all actions
		if (actionsToRun.includes("*")) {
			actionsToRun = [
				...Object.values(this.config.projects).reduce((carry, project) => {
					return new Set([...carry, ...Object.keys(project.actions)]);
				}, new Set<string>()),
			];
		}
		let groupsToRun = groupToRun.split(delimiter);
		// If '*' is in groupsToRun, then we run all groups
		if (groupsToRun.includes("*")) {
			groupsToRun = [
				...Object.values(this.config.projects).reduce((carry, project) => {
					for (const action of actionsToRun) {
						const groups = project.actions[action]?.groups ?? {};
						return new Set([...carry, ...Object.keys(groups)]);
					}
					return carry;
				}, new Set<string>()),
			];
		}
		let projectsToRun = projectToRun ? projectToRun.split(delimiter) : ["*"];
		if (projectsToRun.includes("*")) {
			projectsToRun = Object.keys(this.config.projects);
		}

		return [actionsToRun, groupsToRun, projectsToRun];
	};
}
