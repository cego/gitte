import chalk from "chalk";
import { Config, ProjectAction } from "../types/config";
import { ChildProcessOutput, GroupKey } from "../types/utils";
// import { logActionOutput, searchOutputForHints } from "../search_output";
import { printHeader } from "../utils";
import { getProgressBar, waitingOnToString } from "../progress";
import { SingleBar } from "cli-progress";
import fs from "fs-extra";
import path from "path";
import { Writable } from "stream";
import ansiEscapes from "ansi-escapes";
import ON_DEATH from "death";
import assert, { AssertionError } from "assert";
import { getProjectDirFromRemote } from "../project";
import { Task } from "../task_running/task";
import { TaskPlanner } from "./task_planner";
import { TaskRunner } from "./task_runner";

/** The progress bar does not like to output stuff is isTTY is not set to true. */
class BufferStreamWithTty extends Writable {
	isTTY = true;
}

/**
 * This class is responsible for calling task planner and runner.
 * It handles the output of the runner
 */
class TaskHandler {
    private readonly maxLines = 10;
	private lastFewLines: { out: string; project: string }[] = [];
	private progressBar?: SingleBar;
	private readonly config: Config;
	private waitingOn = [] as GroupKey[];
	private termBuffer = "";
	private bufferStream?: BufferStreamWithTty;
    private plan: Task[];
	private runString: string;

	constructor(cfg: Config, actionToRun: string, groupToRun: string, projectToRun: string) {
		this.config = cfg;
        this.plan = (new TaskPlanner(cfg)).planStringInput(actionToRun, groupToRun, projectToRun);
		this.runString = `${actionToRun} ${groupToRun} ${projectToRun}`;
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

		// Remove cursor restore
		this.termBuffer = this.termBuffer.replace("\u001b8", "");

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

	beganTask = (task: Task): boolean => {
		this.waitingOn.push(task.key);
		this.progressBar?.update({ status: waitingOnToString(this.waitingOn.map(key => `${key.action}/${key.project}/${key.group}`)) });

		return true;
	};

	finishedTask = (task: Task) => {
		this.waitingOn = this.waitingOn.filter((key) => key !== task.key);
		this.progressBar?.increment({ status: waitingOnToString(this.waitingOn.map(key => `${key.action}/${key.project}/${key.group}`)) });
	};

	/**
	 * Called by action runner, should not be called anywhere else.
	 * @param actionsToRun
	 */
	init = (actionsToRun: (GroupKey & ProjectAction)[]): void => {
		this.progressBar?.start(actionsToRun.length, 0, { status: waitingOnToString([]) });
	};

	run = async (): Promise<void> => {
        const taskRunner = new TaskRunner(this.plan, this)
		
        // 1. Prepare output for running.
        const addToBufferStream = this.addToBufferStream;
		this.bufferStream = new BufferStreamWithTty({
			write(chunk, _, callback) {
				addToBufferStream(chunk.toString());
				callback();
			},
		});
		this.progressBar = getProgressBar(`Running ${this.runString}`, this.bufferStream);

		printHeader(`Running ${this.runString}`);
		this.prepareOutputLines();
		// every 100ms, print output
		const interval = setInterval(() => {
			this.printOutputLines();
		}, 100);
		
        // 2. Run
        await taskRunner.run();

        // 3. Cleanup output
		clearInterval(interval);
		this.progressBar.update({ status: waitingOnToString(null) });
		this.progressBar.stop();
		this.printOutputLines();
		await this.clearOutputLines();

        // 4. Print summary
        printHeader(`TODO Summary`);

		// assert(!isError, "At least one action failed");
	
	};

	static getLogFilePath = async (cwd: string, log: GroupKey & ChildProcessOutput): Promise<string> => {
		const logsFolderPath = path.join(cwd, "logs");

		if (!(await fs.pathExists(logsFolderPath))) {
			await fs.mkdir(logsFolderPath);
		}

		return path.join(logsFolderPath, `${log.action}-${log.group}-${log.project}.log`);
	};

	stashLogsToFile = async (logs: (GroupKey & ChildProcessOutput)[]) => {
		for (const log of logs) {
			const logsFilePath = await TaskHandler.getLogFilePath(this.config.cwd, log);
			const output = [];
			output.push(...(log.stdout?.split("\n").map((line) => `[stdout] ${line.trim()}`) ?? []));
			output.push(...(log.stderr?.split("\n").map((line) => `[stderr] ${line.trim()}`) ?? []));
			output.push(
				`[exitCode] ${log.cmd?.join(" ")} exited with ${log.exitCode} in ${log.dir} at ${new Date().toISOString()}`,
			);
			await fs.writeFile(logsFilePath, output.join("\n"));
		}
	};
}

export { TaskHandler }