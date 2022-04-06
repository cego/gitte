import { CmdAction, Config, Project, ShellAction } from "../../src/types/config";

export const projectStub: Project = {
	default_branch: "main",
	remote: "git@gitlab.com:cego/example.git",
	actions: {
		start: {
			groups: {
				"cego.dk": ["docker-compose", "up"],
				"example.com": ["scp", "user@example.com", "sh", "-c", "service", "webserver", "start"],
			},
		},
		down: {
			groups: {
				"cego.dk": ["docker-compose", "down"],
				"example.com": ["scp", "user@example.com", "sh", "-c", "service", "webserver", "stop"],
			},
		},
	},
};
export const startupStub: { [key: string]: CmdAction | ShellAction } = {
	world: { cmd: ["echo", "world"] },
	bashWorld: { shell: "bash", script: "echo world" },
};
export const cnfStub: Config = {
	cwd: "/home/user/gitte",
	startup: startupStub,
	projects: {
		projecta: projectStub,
		projectd: {
			default_branch: "main",
			remote: "git@gitlab.com:cego/exampled.git",
			actions: {
				up: {
					needs: ["projecte"],
					groups: {
						"cego.dk": ["docker-compose", "up"],
					},
				},
			},
		},
		projecte: {
			default_branch: "main",
			remote: "git@gitlab.com:cego/exampled.git",
			actions: {
				up: {
					needs: [],
					groups: {
						"cego.dk": ["docker-compose", "up"],
					},
				},
			},
		},
	},
	searchFor: [],
};
export const cwdStub = "/home/user/gitte";
