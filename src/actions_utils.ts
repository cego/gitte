import chalk from "chalk";
import { Config, ProjectAction } from "./types/config";
import * as pcp from "promisify-child-process";
import { GroupKey } from "./types/utils";
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

class BufferStreamWithTty extends Writable {
	isTTY = true;
}

export class ActionOutputPrinter {
	maxLines = 10;
	lastFewLines: { out: string; project: string }[] = [];
	progressBar: SingleBar;
	actionToRun: string;
	groupToRun: string;
	config: Config;
	waitingOn = [] as string[];
	termBuffer = "";
	bufferStream: BufferStreamWithTty;

	constructor(cfg: Config, actionToRun: string, groupToRun: string) {
		this.actionToRun = actionToRun;
		this.groupToRun = groupToRun;
		this.config = cfg;
		const addToBufferStream = this.addToBufferStream;
		this.bufferStream = new BufferStreamWithTty({
			write(chunk, _, callback) {
				addToBufferStream(chunk.toString());
				callback();
			},
		});
		this.progressBar = getProgressBar(`Running ${actionToRun} ${groupToRun}`, this.bufferStream);
	}

	addToBufferStream = (chunk: string) => {
		this.termBuffer += chunk;
	};

	printOutputLines = () => {
		process.stdout.write(ansiEscapes.cursorUp(this.maxLines + 1));
		process.stdout.write(this.termBuffer);
		process.stdout.write(ansiEscapes.cursorNextLine);
		for (let i = 0; i < this.maxLines; i++) {
			process.stdout.write(ansiEscapes.cursorNextLine);

			process.stdout.write(
				this.lastFewLines[i]
					? chalk`{inverse  ${this.lastFewLines[i].project} } {gray ${this.lastFewLines[i].out}}`
					: ``,
			);
			process.stdout.write(ansiEscapes.eraseEndLine);
		}
		process.stdout.write(ansiEscapes.cursorDown(this.maxLines + 1));
	};

	handleLogOutput = (str: string, projectName: string) => {
		// Remove all ansi color and cursor codes
		// eslint-disable-next-line no-control-regex
		str = str.replace(/\u001b[^m]*?m/g, "").replace(/\x1B(?:[@-Z\\-_]|\[[0-?]*[ -/]*[@-~])/g, "");

		const lines = str
			.split("\n")
			.map((splitted) => splitted.replace(/\n/g, ""))
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
		process.stdout.write(ansiEscapes.cursorUp(this.maxLines));
		process.stdout.write(ansiEscapes.eraseDown);
		process.stdout.write(ansiEscapes.cursorNextLine);
	};
	prepareOutputLines = () => {
		const showCursor = () => {
			process.stdout.write(ansiEscapes.cursorShow);
		};
		process.on("exit", showCursor);
		ON_DEATH(showCursor);
		process.stdout.write(ansiEscapes.cursorHide);

		process.stdout.write("\n");
		for (let i = 0; i < this.maxLines; i++) {
			process.stdout.write("\n");
		}
	};

	beganTask = (project: string) => {
		this.waitingOn.push(project);
		this.progressBar.update({ status: waitingOnToString(this.waitingOn) });
	};

	finishedTask = (project: string) => {
		this.waitingOn = this.waitingOn.filter((p) => p !== project);
		this.progressBar.increment({ status: waitingOnToString(this.waitingOn) });
	};

	/**
	 * Called by action runner, should not be called anywhere else.
	 * @param actionsToRun
	 */
	init = (actionsToRun: (GroupKey & ProjectAction)[]): void => {
		this.bufferStream.write("awdawd");
		this.progressBar.start(actionsToRun.length, 0, { status: waitingOnToString([]) });
	};

	run = async (): Promise<void> => {
		printHeader("Running actions");
		this.prepareOutputLines();
		// every 100ms, print output
		const interval = setInterval(() => {
			this.printOutputLines();
		}, 100);
		const stdoutBuffer: (GroupKey & pcp.Output)[] = await actions(this.config, this.actionToRun, this.groupToRun, this);
		clearInterval(interval);
		this.progressBar.update({ status: waitingOnToString([]) });
		this.progressBar.stop();
		// final flush
		this.printOutputLines();
		await this.clearOutputLines();
		logActionOutput(stdoutBuffer);
		if (this.config.searchFor) searchOutputForHints(this.config, stdoutBuffer);
		if (stdoutBuffer.length === 0) {
			console.log(
				chalk`{yellow No groups found for action {cyan ${this.actionToRun}} and group in {cyan ${this.groupToRun}}}`,
			);
		}
		fs.writeFileSync(path.join(this.config.cwd, ".output.json"), JSON.stringify(stdoutBuffer));
	};
}
