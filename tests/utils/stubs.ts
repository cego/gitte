import {
	CmdAction,
	Config,
	Project,
	ShellAction,
} from "../../src/types/config";

export const projectStub: Project = {
	default_branch: "main",
	remote: "git@gitlab.com:cego/example.git",
	priority: 0,
	actions: {
		start: {
			groups: {
				"cego.dk": ["docker-compose", "up"],
				"example.com": [
					"scp",
					"user@example.com",
					"sh",
					"-c",
					"service",
					"webserver",
					"start",
				],
			},
		},
		down: {
			groups: {
				"cego.dk": ["docker-compose", "down"],
				"example.com": [
					"scp",
					"user@example.com",
					"sh",
					"-c",
					"service",
					"webserver",
					"stop",
				],
			},
		},
	},
};
export const startupStub: { [key: string]: CmdAction | ShellAction } = {
	world: { cmd: ["echo", "world"] },
	bashWorld: { shell: "bash", script: "echo world" },
};
export const cnfStub: Config = {
	startup: startupStub,
	projects: {
		projecta: projectStub,
	},
	searchFor: [],
};
export const cwdStub = "/home/user/git-local-devops";
