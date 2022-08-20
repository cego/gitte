import { fromConfig, hasLocalChanges } from "./gitops";
import { getProjectDirFromRemote } from "./project";
import { Config } from "./types/config";
import * as utils from "./utils";
import fs from "fs-extra";
import path from "path";
import chalk from "chalk";
import tildify from "tildify";

type ShouldDeleteFolder = {
	keep: boolean;
	foldersToDelete: string[];
};

class GitteCleaner {
	readonly allowedFolders: string[] = ["logs"];

	constructor(private config: Config) {}

	async clean() {
		await this.cleanUntracked();
		await this.cleanLocalChanges();
		await this.cleanMaster();
		await this.cleanNonGitte();
		await fromConfig(this.config, false);
	}

	async cleanUntracked() {
		const gitFolders = this.getGitFolders();
		utils.printHeader(`Cleaning untracked files..`);
		for (const project of gitFolders) {
			try {
				await utils.spawn("git", ["clean", "-fdx"], { cwd: project.cwd });
			} catch (e) {
				console.error(
					chalk`{red Failed to clean untracked files in {cyan ${tildify(
						project.cwd,
					)}}. Try running {cyan git clean -fdx} manually}`,
				);
			}
		}
	}

	async cleanLocalChanges() {
		const gitFolders = this.getGitFolders();
		utils.printHeader(`Cleaning local changes..`);
		for (const project of gitFolders) {
			if (await hasLocalChanges(project.cwd)) {
				if (
					await utils.promptBoolean(
						`There are local changes in ${tildify(project.cwd)}. Are you sure you want to delete them?`,
					)
				) {
					try {
						await utils.spawn("git", ["reset", "--hard"], { cwd: project.cwd });
					} catch (e) {
						console.error(
							chalk`{red Failed to clean local changes in {cyan ${tildify(
								project.cwd,
							)}}. Try running {cyan git reset --hard} manually}`,
						);
					}
				}
			}
		}
	}

	async cleanMaster() {
		const gitFolders = this.getGitFolders();
		utils.printHeader(`Checking out master..`);
		for (const project of gitFolders) {
			try {
				await utils.spawn("git", ["checkout", project.defaultBranch], { cwd: project.cwd });
			} catch (e) {
				console.error(
					chalk`{red Failed to checkout ${project.defaultBranch} in {cyan ${tildify(
						project.cwd,
					)}}. Try running {cyan git checkout ${project.defaultBranch}} manually}`,
				);
			}
		}
	}

	async cleanNonGitte() {
		const gitFolders = this.getGitFolders();
		const gitteFolder = this.config.cwd;
		utils.printHeader(`Cleaning non-gitte files..`);
		const res = await this.cleanFolder(gitteFolder, gitFolders);
		if (res.foldersToDelete.length > 0) {
			console.log("Going to delete the following folders, which are not maintained by gitte:");
			res.foldersToDelete.forEach((folder) => {
				console.log(`  - ${tildify(folder)}`);
			});

			if (await utils.promptBoolean("Are you sure you want to delete these folders?")) {
				for (const folder of res.foldersToDelete) {
					fs.removeSync(folder);
				}
			}
		} else {
			console.log(chalk`{green No non-gitte maintained folders found}`);
		}
	}

	private async cleanFolder(
		cwd: string,
		gitFolders: { cwd: string; defaultBranch: string }[],
	): Promise<ShouldDeleteFolder> {
		const value: ShouldDeleteFolder = { keep: false, foldersToDelete: [] };
		// get all contents of cwd
		const contents = await fs.readdir(cwd);
		for (const content of contents) {
			const contentPath = path.join(cwd, content);

			if (!(await fs.lstat(contentPath).then((stat) => stat.isDirectory()))) {
				continue;
			}

			// if folder starts with .
			if (content.startsWith(".")) {
				continue;
			}

			// if folder is in allowed folders
			if (this.allowedFolders.includes(path.relative(this.config.cwd, contentPath))) {
				continue;
			}

			// if folder is in gitFolders
			if (gitFolders.find((folder) => folder.cwd === contentPath)) {
				value.keep = true;
				continue;
			}
			// if folder is not in gitFolders
			const folderContainsGitteContent = await this.cleanFolder(contentPath, gitFolders);
			if (!folderContainsGitteContent.keep) {
				value.foldersToDelete = [...value.foldersToDelete, contentPath];
			} else {
				value.foldersToDelete = [...value.foldersToDelete, ...folderContainsGitteContent.foldersToDelete];
			}
			value.keep = folderContainsGitteContent.keep || value.keep;
		}

		return value;
	}

	private getGitFolders(): { cwd: string; defaultBranch: string }[] {
		return Object.values(this.config.projects)
			.map((project) => {
				return {
					cwd: getProjectDirFromRemote(this.config.cwd, project.remote),
					defaultBranch: project.default_branch,
				};
			})
			.filter((project) => fs.existsSync(project.cwd));
	}
}

export { GitteCleaner };
