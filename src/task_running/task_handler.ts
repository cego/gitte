import chalk from "chalk";
import { Config } from "../types/config";
import { compareGroupKeys, printHeader } from "../utils";
import { getProgressBar, waitingOnToString } from "../progress";
import { SingleBar } from "cli-progress";
import { Writable } from "stream";
import ansiEscapes from "ansi-escapes";
import ON_DEATH from "death";
import assert from "assert";
import { Task } from "../task_running/task";
import { TaskPlanner } from "./task_planner";
import { TaskRunner } from "./task_runner";
import { logTaskOutput, searchOutputForHints, stashLogsToFile } from "../search_output";

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
	private lastFewLines: { out: string; task: Task }[] = [];
	private progressBar?: SingleBar;
	private readonly config: Config;
	private waitingOn = [] as Task[];
	private termBuffer = "";
	private bufferStream?: BufferStreamWithTty;
	private plan: Task[];
	private actions: string[];

	constructor(cfg: Config, actionToRun: string, groupToRun: string, projectToRun: string) {
		this.config = cfg;
		this.plan = new TaskPlanner(cfg).planStringInput(actionToRun, groupToRun, projectToRun);
		this.actions = this.getActionsInOrderFromActionString(actionToRun);
	}

	getActionsInOrderFromActionString(actionsString: string) {
		const actions = actionsString.split("+");

		if (actions.includes("*")) {
			return [...this.plan.reduce((carry, task) => new Set([...carry, ...task.key.action]), new Set<string>())];
		}

		return actions;
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
				const keyString = this.lastFewLines[i].task.toString();
				const maxWidth = Math.max(width - (keyString.length + 3), 0);
				toWrite += chalk`{inverse  ${keyString} } {gray ${this.lastFewLines[i].out.slice(0, maxWidth)}}`;
			}
		}
		process.stdout.write(toWrite);
	};

	handleLogOutput = (str: string, task: Task) => {
		// Only print "printable" characters
		str = str.replace(/[\p{Cc}\p{Cf}\p{Cs}]+/gu, "");

		const lines = str
			.split("\n")
			.map((splitted) => splitted.replace(/\r/g, ""))
			.filter((splitted) => splitted.length);
		lines.forEach((line) => {
			this.lastFewLines.push({ out: line, task });
		});

		while (this.lastFewLines.length > this.maxLines) {
			this.lastFewLines.shift();
		}
	};

	getWritableStream = (task: Task) => {
		const handle = this.handleLogOutput;
		return new Writable({
			write(chunk, _, callback) {
				handle(chunk.toString(), task);
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
		this.waitingOn.push(task);
		this.progressBar?.update({
			status: waitingOnToString(this.waitingOn.map((task) => `${task.toString()}`)),
		});

		return true;
	};

	finishedTask = (task: Task) => {
		this.waitingOn = this.waitingOn.filter((taskWaitingOn) => !compareGroupKeys(taskWaitingOn.key, task.key));
		this.progressBar?.increment({
			status: waitingOnToString(this.waitingOn.map((task) => `${task.toString()}`)),
		});
	};

	run = async () => {
		for (const action of this.actions) {
			await this.runAction(action);
		}
	};

	runAction = async (action: string): Promise<void> => {
		this.lastFewLines = [];
		const taskRunner = new TaskRunner(this.plan, this, action);

		// 1. Prepare output for running.
		const addToBufferStream = this.addToBufferStream;
		this.bufferStream = new BufferStreamWithTty({
			write(chunk, _, callback) {
				addToBufferStream(chunk.toString());
				callback();
			},
		});
		this.progressBar = getProgressBar(`Running ${action}`, this.bufferStream);

		printHeader(`Running ${action}`);
		this.prepareOutputLines();
		// every 100ms, print output
		const interval = setInterval(() => {
			this.printOutputLines();
		}, 100);
		this.progressBar?.start(taskRunner.tasks.length, 0, { status: waitingOnToString([]) });

		// 2. Run
		await taskRunner.run();

		// 3. Cleanup output
		clearInterval(interval);
		this.progressBar.update({ status: waitingOnToString(null) });
		this.progressBar.stop();
		this.printOutputLines();
		await this.clearOutputLines();

		// 4. Print summary
		const isError = await logTaskOutput(this.plan, this.config.cwd, action);
		searchOutputForHints(this.plan, this.config, action);
		stashLogsToFile(this.plan, this.config, action);

		assert(!isError, "At least one action failed");
	};
}

export { TaskHandler };
